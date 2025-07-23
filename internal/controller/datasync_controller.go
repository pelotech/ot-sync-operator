package controller

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	crdv1 "pelotech/ot-sync-operator/api/v1"
	controllerutil "pelotech/ot-sync-operator/internal/contoller-utils"
	generalutils "pelotech/ot-sync-operator/internal/general-utils"
	resourcemanager "pelotech/ot-sync-operator/internal/resource-manager"
)

// DataSyncReconciler reconciles a DataSync object
type DataSyncReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Recorder        record.EventRecorder
	ResourceManager resourcemanager.ResourceManager[crdv1.DataSync]
}

// RBAC for our CRD
// +kubebuilder:rbac:groups=crd.pelotech.ot,resources=datasyncs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=crd.pelotech.ot,resources=datasyncs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=crd.pelotech.ot,resources=datasyncs/finalizers,verbs=update

// RBAC so we can watch configmaps in the cluster from the controller
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

// RBAC preform CRUD operations on pvcs, datavolumes and volumesnapshots
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cdi.kubevirt.io,resources=datavolumes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile

const requeueTimeInveral = 10 * time.Second

const defaultConcurrancyLimit = 4
const defaultRetryLimit = 2
const defaultBackoffLimit = 120 * time.Second

func (r *DataSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	var dataSync crdv1.DataSync
	if err := r.Get(ctx, req.NamespacedName, &dataSync); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("DataSync resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get DataSync")
		return ctrl.Result{}, err
	}

	configMapName := "datasync-operator-config"
	operatorConfigmap, err := generalutils.GetConfigMap(ctx, r.Client, configMapName, dataSync.Namespace)
	if err != nil {
		logger.Error(err, "Failed to get operator config")
	}

	var controllerConfig *controllerutil.OperatorConfig
	controllerConfig, err = controllerutil.ExtractOperatorConfig(operatorConfigmap)

	if err != nil {
		logger.Info("Failed to get operator config using default")
		controllerConfig = &controllerutil.OperatorConfig{
			Concurrency:          defaultConcurrancyLimit,
			RetryLimit:           defaultRetryLimit,
			RetryBackoffDuration: defaultBackoffLimit,
		}
	}

	currentPhase := dataSync.Status.Phase
	logger.Info("Reconciling DataSync", "Phase", currentPhase, "Name", dataSync.Name)

	switch currentPhase {
	case "":
		return r.queueResourceCreation(ctx, &dataSync)
	case crdv1.DataSyncPhaseQueued:
		return r.attemptSyncingOfResource(ctx, &dataSync, controllerConfig.Concurrency)
	case crdv1.DataSyncPhaseSyncing:
		return r.transitonFromSyncing(ctx, &dataSync, controllerConfig.RetryLimit, controllerConfig.RetryBackoffDuration)
	case crdv1.DataSyncPhaseCompleted, crdv1.DataSyncPhaseFailed:
		logger.Info("Resource is in a terminal state, no action needed.")
		return ctrl.Result{}, nil
	default:
		logger.Error(nil, "Unknown phase detected", "Phase", currentPhase)
		return ctrl.Result{}, nil
	}
}

func (r *DataSyncReconciler) queueResourceCreation(ctx context.Context, ds *crdv1.DataSync) (ctrl.Result, error) {
	ds.Status.Phase = crdv1.DataSyncPhaseQueued
	ds.Status.Message = "Request is waiting for an available worker."
	meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
		Type:    crdv1.DataSyncTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  "Queued",
		Message: "The sync has been queued for processing.",
	})

	if err := r.Status().Update(ctx, ds); err != nil {
		return r.handleResourceUpdateError(ctx, ds, err, "Failed to update status to Queued")
	}

	r.Recorder.Eventf(ds, "Normal", "Queued", "Resource successfully queued for sync orchestration")

	return ctrl.Result{Requeue: true}, nil
}

func (r *DataSyncReconciler) attemptSyncingOfResource(ctx context.Context, ds *crdv1.DataSync, syncLimit int) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	syncingList, err := controllerutil.ListDataSyncsByPhase(ctx, r.Client, crdv1.DataSyncPhaseSyncing)

	if err != nil {
		logger.Error(err, "Failed to list syncing resources")
		return ctrl.Result{}, err
	}

	// If the limit is reached, requeue and wait
	if len(syncingList.Items) >= syncLimit {
		r.Recorder.Eventf(ds, "Normal", "WaitingToSync", "No more than %d DataSyncs can be syncing at once. Waiting...", syncLimit)
		return ctrl.Result{RequeueAfter: requeueTimeInveral}, nil
	}

	err = r.ResourceManager.CreateResources(ctx, r.Client, ds)

	if err != nil {
		r.Recorder.Eventf(ds, "Warning", "ResourceCreationFailed", "Failed to create resources.")
		return r.handleResourceCreationError(ctx, ds, err)
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
		return r.handleResourceUpdateError(ctx, ds, err, "Failed to update status to Syncing")
	}

	r.Recorder.Eventf(ds, "Normal", "SyncStarted", "Resource sync has started")

	return ctrl.Result{}, nil
}

func (r *DataSyncReconciler) transitonFromSyncing(ctx context.Context, ds *crdv1.DataSync, retryLimit int, retryBackoff time.Duration) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	// Check if there is an error occurring in the sync
	syncError := controllerutil.SyncErrorOccurred(ds)

	if syncError != nil {
		logger.Error(syncError, "A sync error has occurred.")
		return r.handleSyncError(ctx, ds, syncError, "A error has occurred while syncing", retryLimit, retryBackoff)
	}

	// Check if the sync is done is not done
	isDone := controllerutil.SyncIsComplete(ds)

	if !isDone {
		logger.Info("Sync is not complete. Requeueing.")
		return ctrl.Result{RequeueAfter: requeueTimeInveral}, nil
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
		return r.handleResourceUpdateError(ctx, ds, err, "Failed to update status to Completed")
	}

	r.Recorder.Eventf(ds, "Normal", "SyncCompleted", "Resource sync completed successfully")

	return ctrl.Result{}, nil
}

func (r *DataSyncReconciler) handleResourceUpdateError(ctx context.Context, ds *crdv1.DataSync, originalErr error, message string) (ctrl.Result, error) {
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

	if err := r.Status().Update(ctx, ds); err != nil {
		logger.Error(err, "Could not update status to Failed after an initial update error")
	}

	return ctrl.Result{}, originalErr
}

func (r *DataSyncReconciler) handleResourceCreationError(ctx context.Context, ds *crdv1.DataSync, originalErr error) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.Error(originalErr, "Failed to create a resource when trying to intiate resource sync")

	r.Recorder.Eventf(ds, "Warning", "ResourceCreationFailed", "Failed to create resources.")

	ds.Status.Phase = crdv1.DataSyncPhaseFailed
	ds.Status.Message = "Failed while creating resources: " + originalErr.Error()
	meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
		Type:    crdv1.DataSyncTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  "ResourceCreationFailed",
		Message: originalErr.Error(),
	})

	if err := r.Status().Update(ctx, ds); err != nil {
		logger.Error(err, "Could not update status to Failed resource creation failure")
	}

	err := r.ResourceManager.TearDownAllResources(ctx, r.Client, ds)

	if err != nil {
		logger.Error(err, "Failed to teardown resources.")
	}

	return ctrl.Result{}, originalErr

}

func (r *DataSyncReconciler) handleSyncError(ctx context.Context, ds *crdv1.DataSync, originalErr error, message string, retryLimit int, retryBackoff time.Duration) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.Error(originalErr, message)

	r.Recorder.Eventf(ds, "Warning", "SyncErrorOccurred", originalErr.Error())

	ds.Status.FailureCount += 1

	if err := r.Status().Update(ctx, ds); err != nil {
		logger.Error(err, "Failed to update resource failure count")
	}

	// if we've failed less times than the retry limit then we just go again
	if ds.Status.FailureCount < retryLimit {
		return ctrl.Result{RequeueAfter: retryBackoff}, nil
	}

	r.Recorder.Eventf(ds, "Error", "SyncExceededRetryCount", "The sync has failed beyond the set retry limit of %s", retryLimit)

	// Mark the resource as Failed
	ds.Status.Phase = crdv1.DataSyncPhaseFailed
	ds.Status.Message = "An error occurred durng reconciliation: " + originalErr.Error()
	meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
		Type:    crdv1.DataSyncTypeFailed,
		Status:  metav1.ConditionTrue,
		Reason:  "SyncFailure",
		Message: originalErr.Error(),
	})

	// Attempt to update the status to Failed, but return the original error
	if err := r.Status().Update(ctx, ds); err != nil {
		logger.Error(err, "Could not update status to Failed after an initial update error")
	}

	err := r.ResourceManager.TearDownAllResources(ctx, r.Client, ds)

	if err != nil {
		logger.Error(err, "Failed to teardown resources.")
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
