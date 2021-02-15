package clmobile

import (
	"encoding/base64"
	"encoding/json"
	"github.com/go-errors/errors"
	"github.com/minvws/nl-covid19-coronatester-ctcl-core/holder"
	"github.com/minvws/nl-covid19-coronatester-ctcl-core/verifier"
	"github.com/privacybydesign/gabi"
	"github.com/privacybydesign/gabi/big"
)

type Result struct {
	Value []byte
	Error string
}

func GenerateHolderSk() *Result {
	holderSkJson, err := json.Marshal(holder.GenerateHolderSk())
	if err != nil {
		return &Result{nil, errors.Errorf("Could not serialize holder secret key").Error()}
	}

	return &Result{holderSkJson, ""}
}

// TODO: Handle state properly
var dirtyHack *gabi.CredentialBuilder

func CreateCommitmentMessage(holderSkJson, issuerPkXml, issuerNonceBase64 []byte) *Result {
	holderSk := new(big.Int)
	err := json.Unmarshal(holderSkJson, holderSk)
	if err != nil {
		return &Result{nil, errors.WrapPrefix(err, "Could not unmarshal holder sk", 0).Error()}
	}

	issuerPk, err := gabi.NewPublicKeyFromXML(string(issuerPkXml))
	if err != nil {
		return &Result{nil, errors.WrapPrefix(err, "Could not unmarshal issuer public key", 0).Error()}
	}

	issuerNonce, err := base64DecodeBigInt(issuerNonceBase64)
	if err != nil {
		return &Result{nil, errors.WrapPrefix(err, "Could not unmarshal issuer nonce", 0).Error()}
	}

	credBuilder, icm := holder.CreateCommitment(issuerPk, issuerNonce, holderSk)
	dirtyHack = credBuilder // FIXME

	icmJson, err := json.Marshal(icm)
	if err != nil {
		panic("Could not marshal IssueCommitmentMessage")
	}

	return &Result{icmJson, ""}
}

type CreateCredentialMessage struct {
	IssueSignatureMessage *gabi.IssueSignatureMessage `json:"ism"`
	Attributes            map[string]string           `json:"attributes"`
}

func CreateCredential(holderSkJson, ccmJson []byte) *Result {
	holderSk := new(big.Int)
	err := json.Unmarshal(holderSkJson, holderSk)
	if err != nil {
		return &Result{nil, errors.WrapPrefix(err, "Could not unmarshal holder sk", 0).Error()}
	}

	ccm := &CreateCredentialMessage{}
	err = json.Unmarshal(ccmJson, ccm)
	if err != nil {
		return &Result{nil, errors.WrapPrefix(err, "Could not unmarshal CreateCredentialMessage", 0).Error()}
	}

	credBuilder := dirtyHack // FIXME

	cred, err := holder.CreateCredential(credBuilder, ccm.IssueSignatureMessage, ccm.Attributes)
	if err != nil {
		return &Result{nil, errors.WrapPrefix(err, "Could not create credential", 0).Error()}
	}

	credJson, err := json.Marshal(cred)
	if err != nil {
		return &Result{nil, errors.WrapPrefix(err, "Could not marshal credential", 0).Error()}
	}

	return &Result{credJson, ""}
}

func DiscloseAllWithTime(issuerPkXml, credJson []byte) *Result {
	issuerPk, err := gabi.NewPublicKeyFromXML(string(issuerPkXml))
	if err != nil {
		return &Result{nil, errors.WrapPrefix(err, "Could not unmarshal issuer public key", 0).Error()}
	}

	cred := new(gabi.Credential)
	err = json.Unmarshal(credJson, cred)
	if err != nil {
		return &Result{nil, errors.WrapPrefix(err, "Could not unmarshal credential", 0).Error()}
	}

	cred.Pk = issuerPk

	proofAsn1, err := holder.DiscloseAllWithTime(cred)
	if err != nil {
		return &Result{nil, errors.WrapPrefix(err, "Could not create proof", 0).Error()}
	}

	return &Result{proofAsn1, ""}
}

type VerifyResult struct {
	AttributesJson  []byte
	UnixTimeSeconds int64
	Error           string
}

func Verify(issuerPkXml, proofAsn1 []byte) *VerifyResult {
	issuerPk, err := gabi.NewPublicKeyFromXML(string(issuerPkXml))
	if err != nil {
		return &VerifyResult{nil, 0, errors.WrapPrefix(err, "Could not unmarshal issuer public key", 0).Error()}
	}

	attributes, unixTimeSeconds, err := verifier.Verify(issuerPk, proofAsn1)
	if err != nil {
		return &VerifyResult{nil, 0, errors.WrapPrefix(err, "Could not verify proof", 0).Error()}
	}

	attributesJson, err := json.Marshal(attributes)
	if err != nil {
		return &VerifyResult{nil, 0, errors.WrapPrefix(err, "Could not marshal attributes", 0).Error()}
	}

	return &VerifyResult{attributesJson, unixTimeSeconds, ""}
}

func base64DecodeBigInt(b64 []byte) (*big.Int, error) {
	bts := make([]byte, base64.StdEncoding.DecodedLen(len(b64)))
	n, err := base64.StdEncoding.Decode(bts, b64)
	if err != nil {
		return nil, errors.WrapPrefix(err, "Could not decode bigint", 0)
	}

	i := new(big.Int)
	i.SetBytes(bts[0:n])

	return i, nil
}
