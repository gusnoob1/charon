// Copyright © 2022 Obol Labs Inc.
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the Free
// Software Foundation, either version 3 of the License, or (at your option)
// any later version.
//
// This program is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY; without even the implied warranty of  MERCHANTABILITY or
// FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License for
// more details.
//
// You should have received a copy of the GNU General Public License along with
// this program.  If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	eth2p0 "github.com/attestantio/go-eth2-client/spec/phase0"

	"github.com/obolnetwork/charon/app/errors"
)

// DutyType enumerates the different types of duties.
type DutyType int

const (
	// DutyType enums MUST not change, it will break backwards compatibility.

	DutyUnknown             DutyType = 0
	DutyProposer            DutyType = 1
	DutyAttester            DutyType = 2
	DutySignature           DutyType = 3
	DutyExit                DutyType = 4
	DutyBuilderProposer     DutyType = 5
	DutyBuilderRegistration DutyType = 6
	DutyRandao              DutyType = 7
	// Only ever append new types here...

	dutySentinel DutyType = 8 // Must always be last
)

func (d DutyType) Valid() bool {
	return d > DutyUnknown && d < dutySentinel
}

func (d DutyType) String() string {
	return map[DutyType]string{
		DutyUnknown:             "unknown",
		DutyAttester:            "attester",
		DutyProposer:            "proposer",
		DutyRandao:              "randao",
		DutyExit:                "exit",
		DutyBuilderProposer:     "builder_proposer",
		DutyBuilderRegistration: "builder_registration",
		DutySignature:           "signature",
	}[d]
}

// AllDutyTypes returns a list of all valid duty types.
func AllDutyTypes() []DutyType {
	var resp []DutyType
	for i := DutyUnknown + 1; i.Valid(); i++ {
		resp = append(resp, i)
	}

	return resp
}

// Duty is the unit of work of the core workflow.
type Duty struct {
	// Slot is the Ethereum consensus layer slot.
	Slot int64
	// Type is the duty type performed in the slot.
	Type DutyType
}

func (d Duty) String() string {
	return fmt.Sprintf("%d/%s", d.Slot, d.Type)
}

// NewAttesterDuty returns a new attester duty. It is a convenience function that is
// slightly more readable and concise than the struct literal equivalent:
//
//	core.Duty{Slot: slot, Type: core.DutyAttester}
//	vs
//	core.NewAttesterDuty(slot)
func NewAttesterDuty(slot int64) Duty {
	return Duty{
		Slot: slot,
		Type: DutyAttester,
	}
}

// NewRandaoDuty returns a new randao duty. It is a convenience function that is
// slightly more readable and concise than the struct literal equivalent:
//
//	core.Duty{Slot: slot, Type: core.DutyRandao}
//	vs
//	core.NewRandaoDuty(slot)
func NewRandaoDuty(slot int64) Duty {
	return Duty{
		Slot: slot,
		Type: DutyRandao,
	}
}

// NewProposerDuty returns a new proposer duty. It is a convenience function that is
// slightly more readable and concise than the struct literal equivalent:
//
//	core.Duty{Slot: slot, Type: core.DutyProposer}
//	vs
//	core.NewProposerDuty(slot)
func NewProposerDuty(slot int64) Duty {
	return Duty{
		Slot: slot,
		Type: DutyProposer,
	}
}

// NewVoluntaryExit returns a new voluntary exit duty. It is a convenience function that is
// slightly more readable and concise than the struct literal equivalent:
//
//	core.Duty{Slot: slot, Type: core.DutyExit}
//	vs
//	core.NewVoluntaryExit(slot)
func NewVoluntaryExit(slot int64) Duty {
	return Duty{
		Slot: slot,
		Type: DutyExit,
	}
}

// NewBuilderProposerDuty returns a new builder proposer duty. It is a convenience function that is
// slightly more readable and concise than the struct literal equivalent:
//
//	core.Duty{Slot: slot, Type: core.DutyBuilderProposer}
//	vs
//	core.NewBuilderProposerDuty(slot)
func NewBuilderProposerDuty(slot int64) Duty {
	return Duty{
		Slot: slot,
		Type: DutyBuilderProposer,
	}
}

// NewBuilderRegistrationDuty returns a new builder registration duty. It is a convenience function that is
// slightly more readable and concise than the struct literal equivalent:
//
//	core.Duty{Slot: slot, Type: core.DutyBuilderRegistration}
//	vs
//	core.NewBuilderRegistrationDuty(slot)
func NewBuilderRegistrationDuty(slot int64) Duty {
	return Duty{
		Slot: slot,
		Type: DutyBuilderRegistration,
	}
}

// NewSignatureDuty returns a new Signature duty. It is a convenience function that is
// slightly more readable and concise than the struct literal equivalent:
//
//	core.Duty{Slot: slot, Type: core.DutySignature}
//	vs
//	core.NewSignatureDuty(slot)
func NewSignatureDuty(slot int64) Duty {
	return Duty{
		Slot: slot,
		Type: DutySignature,
	}
}

const (
	pkLen  = 98 // "0x" + hex.Encode([48]byte) = 2+2*48
	sigLen = 96
)

// PubKeyFromBytes returns a new public key from raw bytes.
func PubKeyFromBytes(bytes []byte) (PubKey, error) {
	pk := PubKey(fmt.Sprintf("%#x", bytes))
	if len(pk) != pkLen {
		return "", errors.New("invalid public key length")
	}

	return pk, nil
}

// PubKey is the DV root public key, the identifier of a validator in the core workflow.
// It is a hex formatted string, e.g. "0xb82bc680e...".
type PubKey string

// String returns a concise logging friendly version of the public key, e.g. "b82_97f".
func (k PubKey) String() string {
	if len(k) != pkLen {
		return "<invalid public key:" + string(k) + ">"
	}

	return string(k[2:5]) + "_" + string(k[94:97])
}

// Bytes returns the public key as raw bytes.
func (k PubKey) Bytes() ([]byte, error) {
	if len(k) != pkLen {
		return nil, errors.New("invalid public key length")
	}

	b, err := hex.DecodeString(string(k[2:]))
	if err != nil {
		return nil, errors.Wrap(err, "decode public key hex")
	}

	return b, nil
}

// ToETH2 returns the public key as an eth2 phase0 public key.
func (k PubKey) ToETH2() (eth2p0.BLSPubKey, error) {
	b, err := k.Bytes()
	if err != nil {
		return eth2p0.BLSPubKey{}, err
	}

	var resp eth2p0.BLSPubKey
	copy(resp[:], b)

	return resp, nil
}

// DutyDefinition defines the duty including parameters required
// to fetch the duty data, it is the result of resolving duties
// at the start of an epoch.
type DutyDefinition interface {
	// Clone returns a cloned copy of the DutyDefinition.
	Clone() (DutyDefinition, error)
	// Marshaler returns the json serialised duty definition.
	json.Marshaler
}

// DutyDefinitionSet is a set of duty definitions, one per validator.
type DutyDefinitionSet map[PubKey]DutyDefinition

// Clone returns a cloned copy of the DutyDefinitionSet. For an immutable core workflow architecture,
// remember to clone data when it leaves the current scope (sharing, storing, returning, etc).
func (s DutyDefinitionSet) Clone() (DutyDefinitionSet, error) {
	resp := make(DutyDefinitionSet, len(s))
	for key, data := range s {
		var err error
		resp[key], err = data.Clone()
		if err != nil {
			return nil, err
		}
	}

	return resp, nil
}

// UnsignedData represents an unsigned duty data object.
type UnsignedData interface {
	// Clone returns a cloned copy of the UnsignedData. For an immutable core workflow architecture,
	// remember to clone data when it leaves the current scope (sharing, storing, returning, etc).
	Clone() (UnsignedData, error)
	// Marshaler returns the json serialised unsigned duty data.
	json.Marshaler
}

// UnsignedDataSet is a set of unsigned duty data objects, one per validator.
type UnsignedDataSet map[PubKey]UnsignedData

// Clone returns a cloned copy of the UnsignedDataSet. For an immutable core workflow architecture,
// remember to clone data when it leaves the current scope (sharing, storing, returning, etc).
func (s UnsignedDataSet) Clone() (UnsignedDataSet, error) {
	resp := make(UnsignedDataSet, len(s))
	for key, data := range s {
		var err error
		resp[key], err = data.Clone()
		if err != nil {
			return nil, err
		}
	}

	return resp, nil
}

// SignedData is a signed duty data.
type SignedData interface {
	// Signature returns the signed duty data's signature.
	Signature() Signature
	// SetSignature returns a copy of signed duty data with the signature replaced.
	SetSignature(Signature) (SignedData, error)
	// Clone returns a cloned copy of the SignedData. For an immutable core workflow architecture,
	// remember to clone data when it leaves the current scope (sharing, storing, returning, etc).
	Clone() (SignedData, error)
	// Marshaler returns the json serialised signed duty data (including the signature).
	json.Marshaler
}

// ParSignedData is a partially signed duty data only signed by a single threshold BLS share.
type ParSignedData struct {
	// SignedData is a partially signed duty data.
	SignedData
	// ShareIdx returns the threshold BLS share index.
	ShareIdx int
}

// Clone returns a cloned copy of the ParSignedData. For an immutable core workflow architecture,
// remember to clone data when it leaves the current scope (sharing, storing, returning, etc).
func (d ParSignedData) Clone() (ParSignedData, error) {
	data, err := d.SignedData.Clone()
	if err != nil {
		return ParSignedData{}, err
	}

	return ParSignedData{
		SignedData: data,
		ShareIdx:   d.ShareIdx,
	}, nil
}

// ParSignedDataSet is a set of partially signed duty data objects, one per validator.
type ParSignedDataSet map[PubKey]ParSignedData

// Clone returns a cloned copy of the ParSignedDataSet. For an immutable core workflow architecture,
// remember to clone data when it leaves the current scope (sharing, storing, returning, etc).
func (s ParSignedDataSet) Clone() (ParSignedDataSet, error) {
	resp := make(ParSignedDataSet, len(s))
	for key, data := range s {
		var err error
		resp[key], err = data.Clone()
		if err != nil {
			return nil, err
		}
	}

	return resp, nil
}

// Slot is a beacon chain slot including chain metadata to infer epoch and next slot.
type Slot struct {
	Slot          int64
	Time          time.Time
	SlotDuration  time.Duration
	SlotsPerEpoch int64
}

// Next returns the next slot.
func (s Slot) Next() Slot {
	return Slot{
		Slot:          s.Slot + 1,
		Time:          s.Time.Add(s.SlotDuration),
		SlotsPerEpoch: s.SlotsPerEpoch,
		SlotDuration:  s.SlotDuration,
	}
}

// Epoch returns the epoch of the slot.
func (s Slot) Epoch() int64 {
	return s.Slot / s.SlotsPerEpoch
}

// LastInEpoch returns true if this is the last slot in the epoch.
func (s Slot) LastInEpoch() bool {
	return s.Slot%s.SlotsPerEpoch == s.SlotsPerEpoch-1
}

// FirstInEpoch returns true if this is the first slot in the epoch.
func (s Slot) FirstInEpoch() bool {
	return s.Slot%s.SlotsPerEpoch == 0
}
