package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	crdv1 "pelotech/ot-sync-operator/api/v1"
	dscontrollerutils "pelotech/ot-sync-operator/internal/contoller-utils"
	datasyncservice "pelotech/ot-sync-operator/internal/datasync-service"
	crutils "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// DataSyncReconciler reconciles a DataSync object
type DataSyncReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	datasyncservice.DataSyncService
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

func (r *DataSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	var dataSync crdv1.DataSync

	err := r.Get(ctx, req.NamespacedName, &dataSync)

	if err != nil && errors.IsNotFound(err) {
		logger.Info("Resource has been deleted.")
		return ctrl.Result{}, nil
	}

	if err != nil {
		logger.Error(err, "Failed to get DataSync")
		return ctrl.Result{}, err
	}

	// We don't have our finalizer and haven't been deleted
	if dataSync.GetDeletionTimestamp().IsZero() && !crutils.ContainsFinalizer(&dataSync, crdv1.DataSyncFinalizer) {
		crutils.AddFinalizer(&dataSync, crdv1.DataSyncFinalizer)

		err := r.Update(ctx, &dataSync)

		if err != nil {
			return r.ErrorHandler.HandleResourceUpdateError(ctx, &dataSync, err, "Failed to add finalizer to our resource")
		}

	}

	// We have been deleted with our finalizer
	if !dataSync.GetDeletionTimestamp().IsZero() {
		return r.DataSyncService.DeleteResource(ctx, &dataSync)
	}

	currentPhase := dataSync.Status.Phase
	logger.Info("Reconciling DataSync", "Phase", currentPhase, "Name", dataSync.Name)

	switch currentPhase {
	case "":
		return r.DataSyncService.QueueResourceCreation(ctx, &dataSync)
	case crdv1.DataSyncPhaseQueued:
		return r.DataSyncService.AttemptSyncingOfResource(ctx, &dataSync)
	case crdv1.DataSyncPhaseSyncing:
		return r.DataSyncService.TransitonFromSyncing(ctx, &dataSync)
	case crdv1.DataSyncPhaseCompleted, crdv1.DataSyncPhaseFailed:
		return ctrl.Result{}, nil
	default:
		logger.Error(nil, "Unknown phase detected", "Phase", currentPhase)
		return ctrl.Result{}, nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *DataSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {

	// Index resources by phase since we have to query these quite a bit
	err := mgr.GetFieldIndexer().IndexField(context.Background(), &crdv1.DataSync{}, ".status.phase", dscontrollerutils.IndexDataSyncByPhase)

	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&crdv1.DataSync{}).
		Named("datasync").
		Complete(r)
}
