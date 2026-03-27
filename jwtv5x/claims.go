package jwtv5x

// TokenType distinguishes access tokens from refresh tokens to prevent token confusion attacks.
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)
