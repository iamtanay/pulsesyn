// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title ValidationRegistry
/// @notice Write-once on-chain registry for finalized PulseSyn validation results.
///         Each record is keyed by claim_id. Records are permanent — the
///         immutability of the validation verdict is a core protocol guarantee.
///         See PulseSyn Protocol Specification v0.1, Section 7.2.
contract ValidationRegistry {

    /// @notice On-chain representation of a finalized validation session.
    struct ValidationRecord {
        string  claimID;
        string  verdict;
        /// @dev confidenceScore is scaled by 1e6: 1.0 → 1_000_000.
        ///      Solidity has no native float type.
        uint64  confidenceScore;
        /// @dev merkleRoot is the root of the vote Merkle tree, enabling
        ///      cryptographic inclusion proofs for individual votes.
        bytes32 merkleRoot;
        uint64  epoch;
        uint256 finalizedAt;
    }

    /// @dev Primary storage: keccak256(claimID) → ValidationRecord.
    mapping(bytes32 => ValidationRecord) private _validations;

    /// @notice Emitted when a validation result is recorded on-chain.
    /// @param claimIDHash keccak256 of the protocol claim_id.
    /// @param verdict     The consensus verdict string.
    /// @param epoch       Block number at finalization time.
    event ValidationFinalized(
        bytes32 indexed claimIDHash,
        string          verdict,
        uint64  indexed epoch
    );

    /// @notice Reverts when a write is attempted for an already-recorded claim.
    error ValidationAlreadyExists(bytes32 claimIDHash);

    /// @notice Record a finalized validation. Reverts if already recorded.
    /// @param record The validation record to store.
    function recordValidation(ValidationRecord calldata record) external {
        bytes32 key = keccak256(bytes(record.claimID));
        if (_validations[key].finalizedAt != 0) {
            revert ValidationAlreadyExists(key);
        }
        _validations[key] = record;
        emit ValidationFinalized(key, record.verdict, record.epoch);
    }

    /// @notice Returns the validation record for a given claimID.
    /// @param claimID The protocol-derived claim identifier.
    /// @return The stored ValidationRecord. All fields are zero if not found.
    function getValidation(string calldata claimID)
        external
        view
        returns (ValidationRecord memory)
    {
        return _validations[keccak256(bytes(claimID))];
    }

    /// @notice Returns true if a validation for the given claimID has been recorded.
    /// @param claimID The protocol-derived claim identifier.
    function exists(string calldata claimID) external view returns (bool) {
        return _validations[keccak256(bytes(claimID))].finalizedAt != 0;
    }
}
