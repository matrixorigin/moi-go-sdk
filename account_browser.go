package sdk

import (
	"context"
	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"net/http"
	"net/url"
)

type BrowserLoginCallbackRequest = product.BrowserLoginCallbackRequest
type BrowserSessionResult = product.BrowserSessionResult
type BrowserLogoutRequest = product.BrowserLogoutRequest
type BrowserLogoutResult = product.BrowserLogoutResult
type AuthPreferencesRequest = product.AuthPreferencesRequest
type AuthPreferencesResult = product.AuthPreferencesResult
type NativeBootstrapResult = product.NativeBootstrapResult

// BootstrapNative prepares the product identity for a UC native CLI session.
// Configure WithToken with its access token; cookies and API keys are rejected.
func (s *AccountService) BootstrapNative(ctx context.Context) (*NativeBootstrapResult, error) {
	result := &NativeBootstrapResult{}
	err := s.client.do(ctx, http.MethodPost, "/auth/cli/bootstrap", "", nil, result)
	return result, err
}

// BeginBrowserLogin returns the login redirect and binding cookies without following it.
func (s *AccountService) BeginBrowserLogin(ctx context.Context, returnTo string) (*StreamResponse, error) {
	path := "/auth/login"
	if returnTo != "" {
		path += "?" + url.Values{"return_to": {returnTo}}.Encode()
	}
	return s.client.doStreamURL(ctx, http.MethodGet, s.client.productURL(path), "", nil, acceptAny, true)
}
func (s *AccountService) CompleteBrowserLogin(ctx context.Context, request *BrowserLoginCallbackRequest) (*StreamResponse, error) {
	if request == nil {
		return nil, &Error{Code: "ErrRequestRequired", Message: "browser callback request is required"}
	}
	query := url.Values{"code": {request.GetCode()}, "state": {request.GetState()}, "error": {request.GetError()}}
	return s.client.doStreamURL(ctx, http.MethodGet, s.client.productURL("/auth/callback?"+query.Encode()), "", nil, acceptAny, true)
}
func (s *AccountService) CompleteBrowserLogout(ctx context.Context, state string) (*StreamResponse, error) {
	return s.client.doStreamURL(ctx, http.MethodGet, s.client.productURL("/auth/logout/callback?"+url.Values{"state": {state}}.Encode()), "", nil, acceptAny, true)
}
func (s *AccountService) BrowserSession(ctx context.Context) (*BrowserSessionResult, error) {
	result := &BrowserSessionResult{}
	err := s.client.do(ctx, http.MethodGet, "/auth/me", "", nil, result)
	return result, err
}
func (s *AccountService) LogoutBrowserSession(ctx context.Context, request *BrowserLogoutRequest) (*BrowserLogoutResult, error) {
	if request == nil {
		return nil, &Error{Code: "ErrRequestRequired", Message: "browser logout request is required"}
	}
	result := &BrowserLogoutResult{}
	err := s.client.do(ctx, http.MethodPost, "/auth/logout", "", request, result)
	return result, err
}
func (s *AccountService) Preferences(ctx context.Context) (*AuthPreferencesResult, error) {
	result := &AuthPreferencesResult{}
	err := s.client.do(ctx, http.MethodGet, "/auth/preferences", "", nil, result)
	return result, err
}
func (s *AccountService) UpdatePreferences(ctx context.Context, request *AuthPreferencesRequest) (*AuthPreferencesResult, error) {
	if request == nil {
		return nil, &Error{Code: "ErrRequestRequired", Message: "preferences request is required"}
	}
	result := &AuthPreferencesResult{}
	err := s.client.do(ctx, http.MethodPatch, "/auth/preferences", "", request, result)
	return result, err
}
func (s *AccountService) CheckinStatus(ctx context.Context) (*ProductData, error) {
	result := &ProductData{}
	err := s.client.do(ctx, http.MethodGet, "/user/me/checkin", "", nil, result)
	return result, err
}

func (s *AccountService) RegistrationInvite(ctx context.Context) (*ProductData, error) {
	result := &ProductData{}
	err := s.client.do(ctx, http.MethodGet, "/user/me/registration-invite", "", nil, result)
	return result, err
}

func (s *AccountService) RewardCenterItems(ctx context.Context) (*ProductData, error) {
	result := &ProductData{}
	err := s.client.do(ctx, http.MethodGet, "/user/me/reward-center/items", "", nil, result)
	return result, err
}

func (s *AccountService) RewardCenterEvents(ctx context.Context, eventType string) (*ProductData, error) {
	path := "/user/me/reward-center/events"
	if eventType != "" {
		path += "?" + url.Values{"event_type": {eventType}}.Encode()
	}
	result := &ProductData{}
	err := s.client.do(ctx, http.MethodGet, path, "", nil, result)
	return result, err
}

// RewardCenterInvitations lists the browser's paginated invitation registrations.
func (s *AccountService) RewardCenterInvitations(ctx context.Context, page, pageSize string) (*ProductData, error) {
	path := "/user/me/reward-center/invitations"
	values := url.Values{}
	if page != "" {
		values.Set("page", page)
	}
	if pageSize != "" {
		values.Set("page_size", pageSize)
	}
	if len(values) > 0 {
		path += "?" + values.Encode()
	}
	result := &ProductData{}
	err := s.client.do(ctx, http.MethodGet, path, "", nil, result)
	return result, err
}

// Checkin forwards the browser reward-center source to UC.
func (s *AccountService) Checkin(ctx context.Context, source string) (*ProductData, error) {
	out := &ProductData{}
	err := s.client.do(ctx, http.MethodPost, "/user/me/checkin", "", map[string]string{"source": source}, out)
	return out, err
}
