package auth

// DeviceCodeResponse holds the initial response from a device authorization request.
// It contains the code to show the user and the parameters needed for polling.
type DeviceCodeResponse struct {
	DeviceCode      string
	UserCode        string
	VerificationURI string
	ExpiresIn       int // seconds until the device code expires
	Interval        int // minimum polling interval in seconds
}

// TokenResponse holds the tokens returned after successful OAuth authorization.
type TokenResponse struct {
	AccessToken  string
	RefreshToken string
}
