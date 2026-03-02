package models

type LoginDTO struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type TokenDTO struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}
