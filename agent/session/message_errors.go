package session

type OutputLengthError struct {
	Message string
}

func (e *OutputLengthError) Error() string {
	if e == nil || e.Message == "" {
		return "output length exceeded"
	}
	return e.Message
}

type AuthError struct {
	ProviderID string
	Message    string
}

func (e *AuthError) Error() string {
	if e == nil || e.Message == "" {
		return "authentication error"
	}
	return e.Message
}

type APIError struct {
	Message         string
	StatusCode      int
	IsRetryable     bool
	ResponseBody    string
	ResponseHeaders map[string]string
	Metadata        map[string]string
}

func (e *APIError) Error() string {
	if e == nil || e.Message == "" {
		return "api error"
	}
	return e.Message
}
