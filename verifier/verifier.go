package verifier

import (
	"encoding/asn1"
	"github.com/go-errors/errors"
	"github.com/minvws/nl-covid19-coronacheck-cl-core/common"
	"github.com/privacybydesign/gabi"
	"github.com/privacybydesign/gabi/big"
)

func Verify(issuerPks map[string]*gabi.PublicKey, proofAsn1 []byte) (map[string]string, int64, error) {
	// Deserialize proof
	ps := &common.ProofSerialization{}
	_, err := asn1.Unmarshal(proofAsn1, ps)
	if err != nil {
		return nil, 0, errors.Errorf("Could not deserialize proof")
	}

	// Make sure the amount of disclosure choices match the amount of attributes, plus secret key and metadata
	numAttributes := len(common.AttributeTypes) + 2
	if len(ps.DisclosureChoices) != numAttributes {
		return nil, 0, errors.Errorf("Invalid amount of disclosure choices")
	}

	// Validate that the secret key is not marked as disclosed, and the metadata is marked as disclosed
	if ps.DisclosureChoices[0] {
		return nil, 0, errors.Errorf("First attribute (secret key) should never be disclosed")
	}

	if !ps.DisclosureChoices[1] {
		return nil, 0, errors.Errorf("Second attribute (metadata) should be disclosed")
	}

	// Convert the lists of disclosures and non-disclosure responses to a
	// map from attribute index -> disclosure/response, while checking bounds
	aDisclosed, aResponses := map[int]*big.Int{}, map[int]*big.Int{}

	numDisclosures := len(ps.ADisclosed)
	numResponses := len(ps.AResponses)
	di, ri := 0, 0

	for i, disclosureChoice := range ps.DisclosureChoices {
		if disclosureChoice {
			if di >= numDisclosures {
				return nil, 0, errors.Errorf("Incongruent amount of disclosures")
			}
			aDisclosed[i] = big.Convert(ps.ADisclosed[di])
			di++
		} else {
			if ri >= numResponses {
				return nil, 0, errors.Errorf("Incongruent amount of non-disclosure responses")
			}
			aResponses[i] = big.Convert(ps.AResponses[ri])
			ri++
		}
	}

	// Retrieve the metadata attribute and get the correct public key
	metadataAttribute := common.DecodeAttributeInt(aDisclosed[1])

	credentialMetadata := &common.CredentialMetadataSerialization{}
	_, err = asn1.Unmarshal(metadataAttribute, credentialMetadata)
	if err != nil {
		return nil, 0, errors.Errorf("Could not unmarshal metadata attribute")
	}

	issuerPk, ok := issuerPks[credentialMetadata.IssuerPkId]
	if !ok {
		return nil, 0, errors.Errorf("Credential public key is unknown")
	}

	// Create a proofD structure
	proof := &gabi.ProofD{
		C:          big.Convert(ps.C),
		A:          big.Convert(ps.A),
		EResponse:  big.Convert(ps.EResponse),
		VResponse:  big.Convert(ps.VResponse),
		AResponses: aResponses,
		ADisclosed: aDisclosed,
	}

	var proofList gabi.ProofList
	proofList = append(proofList, proof)

	// Verify with the given timestamp
	timeBasedChallenge := common.CalculateTimeBasedChallenge(ps.UnixTimeSeconds)
	valid := proofList.Verify([]*gabi.PublicKey{issuerPk}, common.BigOne, timeBasedChallenge, false, []string{})

	if !valid {
		return nil, 0, errors.Errorf("Invalid proof")
	}

	// Retrieve attribute values
	attributes := make(map[string]string)
	for disclosureIndex, d := range aDisclosed {
		// Exclude metadata attribute
		if disclosureIndex == 1 {
			continue
		}

		attributeType := common.AttributeTypes[disclosureIndex-2]
		attributes[attributeType] = string(common.DecodeAttributeInt(d))
	}

	return attributes, ps.UnixTimeSeconds, nil
}
