package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// defaultTokenTTL is how long tokens issued by TokenGenerator stay valid.
const defaultTokenTTL = time.Hour

// TokenGenerator issues signed JWTs for tests and local development.
type TokenGenerator struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

// NewTokenGenerator creates a TokenGenerator that signs tokens with jwtSecret.
func NewTokenGenerator(jwtSecret string) *TokenGenerator {
	return &TokenGenerator{
		secret: []byte(jwtSecret),
		issuer: "stellabill",
		ttl:    defaultTokenTTL,
	}
}

// GenerateAdminToken issues a token with the admin role.
func (g *TokenGenerator) GenerateAdminToken(userID, email string) (string, error) {
	return g.generateWithTTL(userID, email, RoleAdmin, g.ttl)
}

// GenerateMerchantToken issues a token with the merchant role and merchant ID.
func (g *TokenGenerator) GenerateMerchantToken(userID, email, merchantID string) (string, error) {
	claims := g.claims(userID, email, RoleMerchant, time.Now().Add(g.ttl))
	claims.MerchantID = merchantID
	return g.sign(claims)
}

// GenerateCustomerToken issues a token with the customer role.
func (g *TokenGenerator) GenerateCustomerToken(userID, email string) (string, error) {
	return g.generateWithTTL(userID, email, RoleCustomer, g.ttl)
}

// GenerateExpiredToken issues an already-expired token with the given role.
func (g *TokenGenerator) GenerateExpiredToken(userID, email string, role Role) (string, error) {
	return g.generateWithTTL(userID, email, role, -time.Hour)
}

// generateWithTTL issues a token with the given role that expires after ttl.
func (g *TokenGenerator) generateWithTTL(userID, email string, role Role, ttl time.Duration) (string, error) {
	return g.sign(g.claims(userID, email, role, time.Now().Add(ttl)))
}

// GenerateTokenWithoutRoles issues a token that carries no role claims.
func (g *TokenGenerator) GenerateTokenWithoutRoles(userID, email string) (string, error) {
	claims := g.claims(userID, email, "", time.Now().Add(g.ttl))
	claims.Role = ""
	claims.Roles = nil
	return g.sign(claims)
}

// GenerateTokenWithoutUserID issues a token with an empty user ID but a valid role.
func (g *TokenGenerator) GenerateTokenWithoutUserID(email string, role Role) (string, error) {
	return g.sign(g.claims("", email, role, time.Now().Add(g.ttl)))
}

// claims builds the base claim set for a generated token.
func (g *TokenGenerator) claims(userID, email string, role Role, exp time.Time) Claims {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    g.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	if role != "" {
		claims.Role = string(role)
		claims.Roles = []string{string(role)}
	}
	return claims
}

// sign serializes and signs claims with the generator's secret.
func (g *TokenGenerator) sign(claims Claims) (string, error) {
	if len(g.secret) == 0 {
		return "", errors.New("token generator requires a non-empty secret")
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(g.secret)
}
