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
)

// Condition types and reasons
const (
	TypeReady string = "Ready"
)

// DataSync Phases
const (
	PhaseNew       string = "New"
	PhaseQueued    string = "Queued"
	PhaseSyncing   string = "Syncing"
	PhaseCompleted string = "Completed"
	PhaseFailed    string = "Failed"
)

// DataSyncReconciler reconciles a DataSync object
type DataSyncReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=crd.pelotech.ot,resources=datasyncs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=crd.pelotech.ot,resources=datasyncs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=crd.pelotech.ot,resources=datasyncs/finalizers,verbs=update

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
	case "": // State: A resource was just created
		return r.reconcileNew(ctx, &dataSync)
	case PhaseNew:
		return r.reconcileQueued(ctx, &dataSync)
	case PhaseQueued:
		return r.reconcileSyncing(ctx, &dataSync)
	case PhaseSyncing:
		return r.reconcileCompleted(ctx, &dataSync)
	case PhaseCompleted, PhaseFailed:
		// Terminal states, do nothing
		logger.Info("Resource is in a terminal state, no action needed.")
		return ctrl.Result{}, nil
	default:
		logger.Error(nil, "Unknown phase detected", "Phase", currentPhase)
		return ctrl.Result{}, nil
	}
}

func (r *DataSyncReconciler) reconcileNew(ctx context.Context, ds *crdv1.DataSync) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.Info("Transitioning to New")

	ds.Status.Phase = PhaseNew
	ds.Status.Message = "New sync request has been acknowledged."
	meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
		Type:    TypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  "New",
		Message: "Sync request is new.",
	})

	if err := r.Status().Update(ctx, ds); err != nil {
		return r.handleUpdateError(ctx, ds, err, "Failed to update status to New")
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *DataSyncReconciler) reconcileQueued(ctx context.Context, ds *crdv1.DataSync) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.Info("Transitioning to Queued")

	ds.Status.Phase = PhaseQueued
	ds.Status.Message = "Request is waiting for an available worker."
	meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
		Type:    TypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  "Queued",
		Message: "The sync has been queued for processing.",
	})

	if err := r.Status().Update(ctx, ds); err != nil {
		return r.handleUpdateError(ctx, ds, err, "Failed to update status to Queued")
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *DataSyncReconciler) reconcileSyncing(ctx context.Context, ds *crdv1.DataSync) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.Info("Transitioning to Syncing")

	ds.Status.Phase = PhaseSyncing
	ds.Status.Message = "Syncing VM data for the workspace."
	meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
		Type:    TypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  "Syncing",
		Message: "The sync is currently in progress.",
	})

	if err := r.Status().Update(ctx, ds); err != nil {
		return r.handleUpdateError(ctx, ds, err, "Failed to update status to Syncing")
	}

	// Simulate work with a 10-second delay
	logger.Info("Simulating sync work for 10 seconds...")
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

// reconcileCompleted handles the transition from Syncing -> Completed
func (r *DataSyncReconciler) reconcileCompleted(ctx context.Context, ds *crdv1.DataSync) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.Info("Transitioning to Completed")

	ds.Status.Phase = PhaseCompleted
	ds.Status.Message = "The data sync completed successfully."
	meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
		Type:    TypeReady,
		Status:  metav1.ConditionTrue,
		Reason:  "Completed",
		Message: "The sync finished successfully.",
	})

	if err := r.Status().Update(ctx, ds); err != nil {
		return r.handleUpdateError(ctx, ds, err, "Failed to update status to Completed")
	}

	return ctrl.Result{}, nil
}

// handleUpdateError centralizes the logic for marking a resource as Failed
func (r *DataSyncReconciler) handleUpdateError(ctx context.Context, ds *crdv1.DataSync, originalErr error, message string) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.Error(originalErr, message)

	// Mark the resource as Failed
	ds.Status.Phase = PhaseFailed
	ds.Status.Message = "An error occurred during reconciliation: " + originalErr.Error()
	meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
		Type:    TypeReady,
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
	return ctrl.NewControllerManagedBy(mgr).
		For(&crdv1.DataSync{}).
		Named("datasync").
		Complete(r)
}
