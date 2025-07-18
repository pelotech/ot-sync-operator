package controller

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	crdv1 "pelotech/ot-sync-operator/api/v1"
	controllerutil "pelotech/ot-sync-operator/internal/contoller-utils"
)

// DataSyncReconciler reconciles a DataSync object
type DataSyncReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=crd.pelotech.ot,resources=datasyncs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=crd.pelotech.ot,resources=datasyncs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=crd.pelotech.ot,resources=datasyncs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch


// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *DataSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	logger := logf.FromContext(ctx)

	// 1. Fetch the DataSync instance
	var dataSync crdv1.DataSync
	if err := r.Get(ctx, req.NamespacedName, &dataSync); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("DataSync resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get DataSync")
		return ctrl.Result{}, err
	}

	// 2. The main reconciliation logic using a state machine
	currentPhase := dataSync.Status.Phase
	logger.Info("Reconciling DataSync", "Phase", currentPhase, "Name", dataSync.Name)

	switch currentPhase {
	case "":
		return r.queueResourceCreation(ctx, &dataSync)
	case crdv1.DataSyncPhaseQueued:
		return r.attemptSyncingOfResource(ctx, &dataSync)
	case crdv1.DataSyncPhaseSyncing:
		return r.transitonFromSyncing(ctx, &dataSync)
	case crdv1.DataSyncPhaseCompleted, crdv1.DataSyncPhaseFailed:
		logger.Info("Resource is in a terminal state, no action needed.")
		return ctrl.Result{}, nil
	default:
		logger.Error(nil, "Unknown phase detected", "Phase", currentPhase)
		return ctrl.Result{}, nil
	}
}

func (r *DataSyncReconciler) queueResourceCreation(ctx context.Context, ds *crdv1.DataSync) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.Info("Transitioning to Queued")

	ds.Status.Phase = crdv1.DataSyncPhaseQueued
	ds.Status.Message = "Request is waiting for an available worker."
	meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
		Type:    crdv1.DataSyncTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  "Queued",
		Message: "The sync has been queued for processing.",
	})

	if err := r.Status().Update(ctx, ds); err != nil {
		return r.markResourceSyncAsFailed(ctx, ds, err, "Failed to update status to Queued")
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *DataSyncReconciler) attemptSyncingOfResource(ctx context.Context, ds *crdv1.DataSync) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	// The Concurrency limit. TODO: Make this come from a watched configmap
	const syncLimit = 2

	syncingList, err := controllerutil.ListDataSyncsByPhase(ctx, r.Client, crdv1.DataSyncPhaseSyncing)

	if err != nil {
		logger.Error(err, "Failed to list syncing resources")
		return ctrl.Result{}, err
	}

	// If the limit is reached, requeue and wait
	if len(syncingList.Items) >= syncLimit {
		logger.Info("Concurrency limit reached, requeueing", "limit", syncLimit, "current", len(syncingList.Items))
		// Requeue after a delay to check again later
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	ds.Status.Phase = crdv1.DataSyncPhaseSyncing
	ds.Status.Message = "Syncing VM data for the workspace."
	meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
		Type:    crdv1.DataSyncTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  "Syncing",
		Message: "The sync is currently in progress.",
	})

	if err := r.Status().Update(ctx, ds); err != nil {
		return r.markResourceSyncAsFailed(ctx, ds, err, "Failed to update status to Syncing")
	}

	return ctrl.Result{}, nil
}

func (r *DataSyncReconciler) transitonFromSyncing(ctx context.Context, ds *crdv1.DataSync) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	// Check if the sync is done is not done
	isDone := controllerutil.SyncIsComplete(ds)

	if !isDone {
		logger.Info("Sync is not complete. Requeueing.")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	ds.Status.Phase = crdv1.DataSyncPhaseCompleted
	ds.Status.Message = "The data sync completed successfully."
	meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
		Type:    crdv1.DataSyncTypeReady,
		Status:  metav1.ConditionTrue,
		Reason:  "Completed",
		Message: "The sync finished successfully.",
	})

	if err := r.Status().Update(ctx, ds); err != nil {
		return r.markResourceSyncAsFailed(ctx, ds, err, "Failed to update status to Completed")
	}

	logger.Info("Sync Complete")
	logger.Info("Transitioning to Completed")
	return ctrl.Result{}, nil
}

func (r *DataSyncReconciler) markResourceSyncAsFailed(ctx context.Context, ds *crdv1.DataSync, originalErr error, message string) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.Error(originalErr, message)

	// Mark the resource as Failed
	ds.Status.Phase = crdv1.DataSyncPhaseFailed
	ds.Status.Message = "An error occurred durng reconciliation: " + originalErr.Error()
	meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
		Type:    crdv1.DataSyncTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  "UpdateError",
		Message: originalErr.Error(),
	})

	// Attempt to update the status to Failed, but return the original error
	if err := r.Status().Update(ctx, ds); err != nil {
		logger.Error(err, "Could not update status to Failed after an initial update error")
	}

	return ctrl.Result{}, originalErr
}

// SetupWithManager sets up the controller with the Manager.
func (r *DataSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {

	// Index resources by phase since we have to query these quite a bit
	err := mgr.GetFieldIndexer().IndexField(context.Background(), &crdv1.DataSync{}, ".status.phase", controllerutil.IndexDataSyncByPhase)

	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&crdv1.DataSync{}).
		Named("datasync").
		Complete(r)
}
