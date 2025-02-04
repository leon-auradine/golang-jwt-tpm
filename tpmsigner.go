package tpmjwt

import (
	"context"
	"crypto"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	jwt "github.com/golang-jwt/jwt"

	"github.com/google/go-tpm-tools/client"
	"github.com/google/go-tpm/tpm2"
)

// Much of this implementation is inspired templated form [gcp-jwt-go](https://github.com/someone1/gcp-jwt-go)

type TPMConfig struct {
	TPMDevice        string
	KeyID            string           // (optional) the TPM keyID (normally the key "Name")
	KeyTemplate      tpm2.Public      // the specifications for the key
	publicKeyFromTPM crypto.PublicKey // the public key from KeyTemplate
}

type tpmConfigKey struct{}

func (k *TPMConfig) GetKeyID() string {
	return k.KeyID
}

func (k *TPMConfig) GetPublicKey() crypto.PublicKey {
	return k.publicKeyFromTPM
}

var (
	SigningMethodTPMRS256 *SigningMethodTPM
	errMissingConfig      = errors.New("tpmjwt: missing configuration in provided context")
	errMissingTPM         = errors.New("tpmjwt: TPM device not available")

	key         client.Key
	handleNames = map[string][]tpm2.HandleType{
		"all":       {tpm2.HandleTypeLoadedSession, tpm2.HandleTypeSavedSession, tpm2.HandleTypeTransient},
		"loaded":    {tpm2.HandleTypeLoadedSession},
		"saved":     {tpm2.HandleTypeSavedSession},
		"transient": {tpm2.HandleTypeTransient},
		"none":      {},
	}

	// Attestation Key
	// https://github.com/google/go-tpm/blob/master/tpm2/constants.go#L152
	AttestationKeyParametersRSA256 = client.AKTemplateRSA()
	// attestationKeyParametersRSA256 = tpm2.Public{
	// 	Type:    tpm2.AlgRSA,
	// 	NameAlg: tpm2.AlgSHA256,
	// 	Attributes: tpm2.FlagSign | tpm2.FlagRestricted | tpm2.FlagFixedTPM |
	// 		tpm2.FlagFixedParent | tpm2.FlagSensitiveDataOrigin | tpm2.FlagUserWithAuth,
	// 	AuthPolicy: []byte{},
	// 	RSAParameters: &tpm2.RSAParams{
	// 		Sign: &tpm2.SigScheme{
	// 			Alg:  tpm2.AlgRSASSA,
	// 			Hash: tpm2.AlgSHA256,
	// 		},
	// 		KeyBits: 2048,
	// 	},
	// }

	// or just an unrestricted key
	UnrestrictedKeyParametersRSA256 = tpm2.Public{
		Type:    tpm2.AlgRSA,
		NameAlg: tpm2.AlgSHA256,
		Attributes: tpm2.FlagFixedTPM | tpm2.FlagFixedParent | tpm2.FlagSensitiveDataOrigin |
			tpm2.FlagUserWithAuth | tpm2.FlagSign,
		AuthPolicy: []byte{},
		RSAParameters: &tpm2.RSAParams{
			Sign: &tpm2.SigScheme{
				Alg:  tpm2.AlgRSASSA,
				Hash: tpm2.AlgSHA256,
			},
			KeyBits: 2048,
		},
	}
)

type SigningMethodTPM struct {
	alg      string
	override jwt.SigningMethod
	hasher   crypto.Hash
}

func loadTPM(device string, flush string) (io.ReadWriteCloser, error) {
	// check if the TPM exists or not
	rwc, err := tpm2.OpenTPM(device)
	if err != nil {
		return nil, fmt.Errorf("tpmjwt: can't open TPM %q: %v", device, err)
	}
	// optionally clear the state
	totalHandles := 0
	for _, handleType := range handleNames[flush] {
		handles, err := client.Handles(rwc, handleType)
		if err != nil {
			defer rwc.Close()
			return nil, fmt.Errorf("tpmjwt: getting handles: %v", err)
		}
		for _, handle := range handles {
			if err = tpm2.FlushContext(rwc, handle); err != nil {
				defer rwc.Close()
				return nil, fmt.Errorf("tpmjwt: error flushing handle 0x%x: %v", handle, err)
			}
			totalHandles++
		}
	}
	return rwc, nil
}

func loadKey(rwc io.ReadWriteCloser, keyTemplate tpm2.Public) (*client.Key, error) {
	return client.NewKey(rwc, tpm2.HandleOwner, keyTemplate)
}

func NewTPMContext(parent context.Context, val *TPMConfig) (context.Context, error) {
	rwc, err := loadTPM(val.TPMDevice, "none")
	if err != nil {
		return nil, fmt.Errorf("tpmjwt: error loading TPM: %v", err)
	}
	defer rwc.Close()

	k, err := client.NewKey(rwc, tpm2.HandleOwner, val.KeyTemplate)
	if err != nil {
		return nil, err
	}

	fmt.Printf("generated new key %x\n", k.Name().Digest.Value)
	defer k.Close()

	val.publicKeyFromTPM = k.PublicKey()
	return context.WithValue(parent, tpmConfigKey{}, val), nil
}

// KMSFromContext extracts a KMSConfig from a context.Context
func TPMFromContext(ctx context.Context) (*TPMConfig, bool) {
	val, ok := ctx.Value(tpmConfigKey{}).(*TPMConfig)
	return val, ok
}

func init() {
	// RS256
	SigningMethodTPMRS256 = &SigningMethodTPM{
		"TPMRS256",
		jwt.SigningMethodRS256,
		crypto.SHA256,
	}
	jwt.RegisterSigningMethod(SigningMethodTPMRS256.Alg(), func() jwt.SigningMethod {
		return SigningMethodTPMRS256
	})
}

// Alg will return the JWT header algorithm identifier this method is configured for.
func (s *SigningMethodTPM) Alg() string {
	return s.alg
}

// Override will override the default JWT implementation of the signing function this Cloud KMS type implements.
func (s *SigningMethodTPM) Override() {
	s.alg = s.override.Alg()
	jwt.RegisterSigningMethod(s.alg, func() jwt.SigningMethod {
		return s
	})
}

func (s *SigningMethodTPM) Hash() crypto.Hash {
	return s.hasher
}

func (s *SigningMethodTPM) Sign(signingString string, key interface{}) (string, error) {
	var ctx context.Context

	switch k := key.(type) {
	case context.Context:
		ctx = k
	default:
		return "", jwt.ErrInvalidKey
	}
	config, ok := TPMFromContext(ctx)
	if !ok {
		return "", errMissingConfig
	}

	rwc, err := loadTPM(config.TPMDevice, "none")
	if err != nil {
		defer rwc.Close()
		return "", err
	}
	defer rwc.Close()
	kk, err := loadKey(rwc, config.KeyTemplate)
	if err != nil {
		return "", err
	}
	defer kk.Close()

	signedBytes, err := kk.SignData([]byte(signingString))

	if err != nil {
		return "", fmt.Errorf("failed to sign data: %v", err)
	}

	return base64.RawURLEncoding.EncodeToString(signedBytes), err
}

func TPMVerfiyKeyfunc(ctx context.Context, config *TPMConfig) (jwt.Keyfunc, error) {
	return func(token *jwt.Token) (interface{}, error) {
		return config.publicKeyFromTPM, nil
	}, nil
}

func (s *SigningMethodTPM) Verify(signingString, signature string, key interface{}) error {
	return s.override.Verify(signingString, signature, key)
}
