package datasyncservice

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	crdv1 "pelotech/ot-sync-operator/api/v1"
	controllerutil "pelotech/ot-sync-operator/internal/contoller-utils"
	dynamicconfigservice "pelotech/ot-sync-operator/internal/dynamic-config-service"
	errorservice "pelotech/ot-sync-operator/internal/error-service"
	resourcemanager "pelotech/ot-sync-operator/internal/resource-manager"
)

const requeueTimeInveral = 10 * time.Second

type IDataSyncService interface {
	QueueResourceCreation(ctx context.Context, ds *crdv1.DataSync) (ctrl.Result, error)
	AttemptSyncingOfResource(ctx context.Context, ds *crdv1.DataSync, syncLimit int) (ctrl.Result, error)
	TransitonFromSyncing(
		ctx context.Context,
		ds *crdv1.DataSync,
		opConfig dynamicconfigservice.OperatorConfig,
	) (ctrl.Result, error)
	CleanupChildrenOnDeletion(ctx context.Context, ds *crdv1.DataSync) (ctrl.Result, error)
}

type DataSyncService struct {
	client.Client
	Recorder        record.EventRecorder
	ResourceManager resourcemanager.ResourceManager[crdv1.DataSync]
	ErrorHandler    errorservice.ErrorHandlerService
}

func (s *DataSyncService) QueueResourceCreation(ctx context.Context, ds *crdv1.DataSync) (ctrl.Result, error) {
	ds.Status.Phase = crdv1.DataSyncPhaseQueued
	ds.Status.Message = "Request is waiting for an available worker."
	meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
		Type:   crdv1.DataSyncTypeReady,
		Status: metav1.ConditionFalse, Reason: "Queued",
		Message: "The sync has been queued for processing.",
	})

	if err := s.Status().Update(ctx, ds); err != nil {
		return s.ErrorHandler.HandleResourceUpdateError(ctx, ds, err, "Failed to update status to Queued")
	}

	s.Recorder.Eventf(ds, "Normal", "Queued", "Resource successfully queued for sync orchestration")

	return ctrl.Result{Requeue: true}, nil
}

func (s *DataSyncService) AttemptSyncingOfResource(
	ctx context.Context,
	ds *crdv1.DataSync,
	syncLimit int,
) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	syncingList, err := controllerutil.ListDataSyncsByPhase(ctx, s.Client, crdv1.DataSyncPhaseSyncing)

	if err != nil {
		logger.Error(err, "Failed to list syncing resources")
		return ctrl.Result{}, err
	}

	if len(syncingList.Items) >= syncLimit {
		s.Recorder.Eventf(ds, "Normal", "WaitingToSync", "No more than %d DataSyncs can be syncing at once. Waiting...", syncLimit)
		return ctrl.Result{RequeueAfter: requeueTimeInveral}, nil
	}

	err = s.ResourceManager.CreateResources(ctx, s.Client, ds)

	if err != nil {
		s.Recorder.Eventf(ds, "Warning", "ResourceCreationFailed", "Failed to create resources: "+err.Error())
		return s.ErrorHandler.HandleResourceCreationError(ctx, ds, err)
	}

	ds.Status.Phase = crdv1.DataSyncPhaseSyncing
	ds.Status.Message = "Syncing VM data for the workspace."
	meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
		Type:    crdv1.DataSyncTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  "Syncing",
		Message: "The sync is currently in progress.",
	})

	if err := s.Status().Update(ctx, ds); err != nil {
		return s.ErrorHandler.HandleResourceUpdateError(ctx, ds, err, "Failed to update status to Syncing")
	}

	orginalDs := ds.DeepCopy()

	now := time.Now().Format(time.RFC3339)

	ds.Annotations[crdv1.SyncStartTimeAnnotation] = now

	if err := s.Client.Patch(ctx, ds, client.MergeFrom(orginalDs)); err != nil {
		return s.ErrorHandler.HandleResourceUpdateError(ctx, ds, err, "Failed to update sync start time")
	}

	s.Recorder.Eventf(ds, "Normal", "SyncStarted", "Resource sync has started")

	return ctrl.Result{}, nil
}

func (s *DataSyncService) TransitonFromSyncing(
	ctx context.Context,
	ds *crdv1.DataSync,
	opConfig dynamicconfigservice.OperatorConfig,
) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	// Check if there is an error occurring in the sync
	syncError := s.ResourceManager.ResourcesHaveErrors(ctx, s.Client, opConfig, ds)

	if syncError != nil {
		logger.Error(syncError, "A sync error has occurred.")
		return s.ErrorHandler.HandleSyncError(ctx, ds, syncError, "A error has occurred while syncing", opConfig.RetryLimit, opConfig.RetryBackoffDuration)
	}

	// Check if the sync is done is not done
	isDone, err := s.ResourceManager.ResourcesAreReady(ctx, s.Client, ds)

	if err != nil {
		logger.Error(err, "Unable to verify if resource is ready or not.")
	}

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

	if err := s.Status().Update(ctx, ds); err != nil {
		return s.ErrorHandler.HandleResourceUpdateError(ctx, ds, err, "Failed to update status to Completed")
	}

	s.Recorder.Eventf(ds, "Normal", "SyncCompleted", "Resource sync completed successfully")

	return ctrl.Result{}, nil
}

func (s *DataSyncService) CleanupChildrenOnDeletion(ctx context.Context, ds *crdv1.DataSync) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	err := s.ResourceManager.TearDownAllResources(ctx, s.Client, ds)

	if err != nil {
		logger.Error(err, "failed to cleanup child resources of datasync.")
	}

	return ctrl.Result{}, nil
}
