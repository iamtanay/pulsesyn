// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title ClaimRegistry
/// @notice Write-once on-chain registry for PulseSyn claim submissions.
///         Each claim is identified by its protocol-derived claim_id.
///         Records are permanent — no updates or deletions are possible.
///         See PulseSyn Protocol Specification v0.1, Section 7.1.
contract ClaimRegistry {

    /// @notice On-chain representation of a submitted claim.
    struct ClaimRecord {
        string  claimID;
        bytes32 contentHash;
        string  submitterID;
        string  claimType;
        string  domain;
        uint64  epoch;
        uint256 submittedAt;
    }

    /// @dev Primary storage: keccak256(claimID) → ClaimRecord.
    mapping(bytes32 => ClaimRecord) private _claims;

    /// @notice Emitted when a claim is registered on-chain.
    /// @param claimIDHash keccak256 of the protocol claim_id.
    /// @param epoch       Block number at submission time.
    /// @param claimType   The claim type string (FACTUAL, CONTEXTUAL, PREDICTIVE).
    event ClaimRegistered(
        bytes32 indexed claimIDHash,
        uint64  indexed epoch,
        string          claimType
    );

    /// @notice Reverts when a write is attempted for an already-registered claim.
    error ClaimAlreadyExists(bytes32 claimIDHash);

    /// @notice Register a new claim. Reverts if the claim already exists.
    /// @param record The claim record to register.
    function registerClaim(ClaimRecord calldata record) external {
        bytes32 key = keccak256(bytes(record.claimID));
        if (_claims[key].submittedAt != 0) {
            revert ClaimAlreadyExists(key);
        }
        _claims[key] = record;
        emit ClaimRegistered(key, record.epoch, record.claimType);
    }

    /// @notice Returns the claim record for a given claimID.
    /// @param claimID The protocol-derived claim identifier.
    /// @return The stored ClaimRecord. All fields are zero if not found.
    function getClaim(string calldata claimID)
        external
        view
        returns (ClaimRecord memory)
    {
        return _claims[keccak256(bytes(claimID))];
    }

    /// @notice Returns true if a claim with the given claimID has been registered.
    /// @param claimID The protocol-derived claim identifier.
    function exists(string calldata claimID) external view returns (bool) {
        return _claims[keccak256(bytes(claimID))].submittedAt != 0;
    }
}
