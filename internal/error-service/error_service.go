package errorservice

import (
	"context"
	"time"

	crdv1 "pelotech/ot-sync-operator/api/v1"
	resourcemanager "pelotech/ot-sync-operator/internal/resource-manager"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type ErrorHandlerService interface {
	HandleResourceUpdateError(ctx context.Context, ds *crdv1.DataSync, originalErr error, message string) (ctrl.Result, error)
	HandleResourceCreationError(ctx context.Context, ds *crdv1.DataSync, originalErr error) (ctrl.Result, error)
	HandleSyncError(ctx context.Context, ds *crdv1.DataSync, originalErr error, message string) (ctrl.Result, error)
}

type ErrorHandler struct {
	Client          client.Client
	Recorder        record.EventRecorder
	ResourceManager resourcemanager.ResourceManager[crdv1.DataSync]
	RetryLimit      int
	RetryBackoff    time.Duration
}

func (e *ErrorHandler) HandleResourceUpdateError(ctx context.Context, ds *crdv1.DataSync, originalErr error, message string) (ctrl.Result, error) {
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

	if err := e.Client.Status().Update(ctx, ds); err != nil {
		logger.Error(err, "Could not update status to Failed after an initial update error")
	}

	return ctrl.Result{}, originalErr
}

func (e *ErrorHandler) HandleResourceCreationError(ctx context.Context, ds *crdv1.DataSync, originalErr error) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.Info("Handling a reousrce creation failure")
	logger.Error(originalErr, "Failed to create a resource when trying to intiate resource sync")

	e.Recorder.Eventf(ds, "Warning", "ResourceCreationFailed", "Failed to create resources.")

	ds.Status.Phase = crdv1.DataSyncPhaseFailed
	ds.Status.Message = "Failed while creating resources: " + originalErr.Error()
	meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
		Type:    crdv1.DataSyncTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  "ResourceCreationFailed",
		Message: originalErr.Error(),
	})

	if err := e.Client.Status().Update(ctx, ds); err != nil {
		logger.Error(err, "Could not update status to Failed resource creation failure")
	}

	err := e.ResourceManager.TearDownAllResources(ctx, e.Client, ds)

	if err != nil {
		logger.Error(err, "Failed to teardown resources.")
	}

	return ctrl.Result{}, originalErr
}

func (e *ErrorHandler) HandleSyncError(ctx context.Context, ds *crdv1.DataSync, originalErr error, message string) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.Error(originalErr, message)

	e.Recorder.Eventf(ds, "Warning", "SyncErrorOccurred", originalErr.Error())

	ds.Status.FailureCount += 1

	if err := e.Client.Status().Update(ctx, ds); err != nil {
		logger.Error(err, "Failed to update resource failure count")
	}

	if ds.Status.FailureCount < e.RetryLimit {
		return ctrl.Result{RequeueAfter: e.RetryBackoff}, nil
	}

	e.Recorder.Eventf(ds, "Warning", "SyncExceededRetryCount", "The sync has failed beyond the set retry limit of %d", e.RetryLimit)

	ds.Status.Phase = crdv1.DataSyncPhaseFailed
	ds.Status.Message = "An error occurred durng reconciliation: " + originalErr.Error()
	meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
		Type:    crdv1.DataSyncTypeFailed,
		Status:  metav1.ConditionTrue,
		Reason:  "SyncFailure",
		Message: originalErr.Error(),
	})

	if err := e.Client.Status().Update(ctx, ds); err != nil {
		logger.Error(err, "Could not update status to Failed after an sync error")
	}

	err := e.ResourceManager.TearDownAllResources(ctx, e.Client, ds)

	if err != nil {
		logger.Error(err, "Failed to teardown resources.")
	}

	return ctrl.Result{}, originalErr
}
