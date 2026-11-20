package sdk

import (
	"context"
	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"net/http"
)

type DataShareRecoveryApproval = product.DataShareRecoveryApproval
type DataShareDDLRecoveryRequest = product.DataShareDDLRecoveryRequest
type DataShareDDLRecoveryResult = product.DataShareDDLRecoveryResult
type DataShareSagaTerminationRequest = product.DataShareSagaTerminationRequest
type DataShareSagaTerminationResult = product.DataShareSagaTerminationResult

func (s *SystemService) ResumeDataShareDDL(ctx context.Context, request *DataShareDDLRecoveryRequest) (*DataShareDDLRecoveryResult, error) {
	endpoint, err := s.client.nativeURL("/api/v1/system/data-share/recovery:resume")
	if err != nil {
		return nil, err
	}
	result := &DataShareDDLRecoveryResult{}
	err = s.client.doURL(ctx, http.MethodPost, endpoint, "", request, result)
	return result, err
}

func (s *SystemService) TerminateDataShareSaga(ctx context.Context, request *DataShareSagaTerminationRequest) (*DataShareSagaTerminationResult, error) {
	endpoint, err := s.client.nativeURL("/api/v1/system/data-share/recovery/terminate")
	if err != nil {
		return nil, err
	}
	result := &DataShareSagaTerminationResult{}
	err = s.client.doURL(ctx, http.MethodPost, endpoint, "", request, result)
	return result, err
}
