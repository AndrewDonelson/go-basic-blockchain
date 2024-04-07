// Package sdk is a software development kit for building blockchain applications.
// File sdk/puid.go - Personal Unique Identifier for all User related Protocol based transactions
package sdk

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
)

var (
	// These are required and will be created when a new blockchain is created
	ThisBlockchainOrganizationID = NewBigInt(0)
	ThisBlockchainAppID          = NewBigInt(0)
	ThisBlockchainAdminUserID    = NewBigInt(0)
	ThisBlockchainDevAssetID     = NewBigInt(0)
	ThisBlockchainMinerID        = NewBigInt(0)
	// ThisBlockchainOrganizationID = NewBigInt(BlockhainOrganizationID)
	// ThisBlockchainAppID          = NewBigInt(BlockchainAppID)
	// ThisBlockchainAdminUserID    = NewBigInt(BlockchainAdminUserID)
)

// New Custom UUID package written in golang from scratch and not using any 3rd party packages.
// PUID format:

type PUID struct {
	//UserID represents a unique user ID and is registered with the blockchain network using an OrganizationID which is optional. If the OrganizationID is not provided, the user will be registered as a personal user.
	UserID BigInt

	//OrganizationID represents a company or organization and is registered with the blockchain network by a personal users PUID
	OrganizationID BigInt

	//AppID represents a unique application ID and is registered with the blockchain network using an OrganizationID and the requesting users PUID (both are required)
	AppID BigInt

	//AssetID represents a unique asset ID and is registered with the blockchain network using a required UserID and optional OrganizationID and AppIDs.
	// If the OrganizationID is not provided, the asset will be registered as a personal asset and not tied to any organization or application.
	// If the AppID is not provided, the asset will be registered as a personal asset and not tied to any application.
	AssetID BigInt
}

// NewPUIDEmpty creates a new PUID instance with all fields set to zero.
func NewPUIDEmpty() *PUID {
	return &PUID{
		UserID:         *NewBigInt(0),
		OrganizationID: *NewBigInt(0),
		AppID:          *NewBigInt(0),
		AssetID:        *NewBigInt(0),
	}
}

// NewPUID creates a new PUID instance.
func NewPUID(organizationID, appID, userID, assetID *BigInt) *PUID {
	return &PUID{
		UserID:         *userID,
		OrganizationID: *organizationID,
		AppID:          *appID,
		AssetID:        *assetID,
	}
}

// NewPUIDFromString creates a new PUID instance from a string representation.
// The string should be in the format: "organizationID:appID:userID:assetID" and can optionally be base64-encoded.
func NewPUIDFromString(puidStr string) (*PUID, error) {

	// Check if the input string is base64-encoded
	if isBase64Encoded(puidStr) {
		// Decode the base64 string
		decoded, err := base64.StdEncoding.DecodeString(puidStr)
		if err != nil {
			return nil, err
		}

		// Create a new PUID from the decoded bytes
		puidStr = string(decoded[:])
	}

	parts := strings.Split(puidStr, ":")
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid PUID string format: %s", puidStr)
	}

	orgID, err := NewBigIntFromString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid organization ID: %w", err)
	}

	appID, err := NewBigIntFromString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid app ID: %w", err)
	}

	userID, err := NewBigIntFromString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	assetID, err := NewBigIntFromString(parts[3])
	if err != nil {
		return nil, fmt.Errorf("invalid asset ID: %w", err)
	}

	return NewPUID(orgID, appID, userID, assetID), nil
}

// Bytes returns the byte representation of the PUID.
func (p *PUID) Bytes() []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, p.UserID.Val)
	binary.Write(buf, binary.BigEndian, p.OrganizationID.Val)
	binary.Write(buf, binary.BigEndian, p.AppID.Val)
	binary.Write(buf, binary.BigEndian, p.AssetID.Val)
	return buf.Bytes()
}

// String returns the string representation of the PUID.
func (p *PUID) String() string {
	return p.UserID.String() + ":" + p.OrganizationID.String() + ":" + p.AppID.String() + ":" + p.AssetID.String()
}

// Base64 returns the base64 representation of the PUID.
func (p *PUID) Base64() string {
	return base64.StdEncoding.EncodeToString(p.Bytes())
}

// GetOrganizationID returns the OrganizationID of the PUID.
func (p *PUID) GetOrganizationID() *BigInt {
	return &p.OrganizationID
}

// GetAppID returns the AppID of the PUID.
func (p *PUID) GetAppID() *BigInt {
	return &p.AppID
}

// GetUserID returns the UserID of the PUID.
func (p *PUID) GetUserID() *BigInt {
	return &p.UserID
}

// GetAssetID returns the AssetID of the PUID.
func (p *PUID) GetAssetID() *BigInt {
	return &p.AssetID
}

// SetAssetID sets the AssetID with a new AssetID of BigInt.
func (p *PUID) SetAssetID(assetID *BigInt) {
	p.AssetID = *assetID
}

// IsOrganiztionID returns true if the PUID has an OrganizationID that matches the provided BigInt value.
func (p *PUID) IsOrganiztionID(organizationID *BigInt) bool {
	return p.OrganizationID.IsEqual(organizationID)
}

// IsAppID returns true if the PUID has an AppID that matches the provided BigInt value.
func (p *PUID) IsAppID(appID *BigInt) bool {
	return p.AppID.IsEqual(appID)
}

// IsUserID returns true if the PUID has an UserID that matches the provided BigInt value.
func (p *PUID) IsUserID(userID *BigInt) bool {
	return p.UserID.IsEqual(userID)
}

// IsAssetID returns true if the PUID has an AssetID that matches the provided BigInt value.
func (p *PUID) IsAssetID(assetID *BigInt) bool {
	return p.AssetID.IsEqual(assetID)
}

// Equal returns true if the given PUID is equal to the current PUID.
func (p *PUID) Equal(other *PUID) bool {
	return p.UserID.IsEqual(&other.UserID) &&
		p.OrganizationID.IsEqual(&other.OrganizationID) &&
		p.AppID.IsEqual(&other.AppID) &&
		p.AssetID.IsEqual(&other.AssetID)
}

// IsZero returns true if the PUID is zero (all fields are zero).
func (p *PUID) IsZero() bool {
	return p.UserID.IsZero() &&
		p.OrganizationID.IsZero() &&
		p.AppID.IsZero() &&
		p.AssetID.IsZero()
}
