// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title ReputationStore
/// @notice On-chain reputation ledger for PulseSyn validators.
///         Stores the latest reputation score per (validatorID, domain) pair.
///         Reputation is NOT write-once — it is updated after each finalized
///         validation session as the self-correcting mechanism of the protocol.
///         See PulseSyn Protocol Specification v0.1, Section 7.3.
contract ReputationStore {

    /// @notice On-chain reputation state for a single (validator, domain) pair.
    struct ReputationRecord {
        string  validatorID;
        string  domain;
        /// @dev score is scaled by 1e6: 1.0 → 1_000_000.
        uint64  score;
        uint64  epoch;
        uint256 updatedAt;
    }

    /// @dev Primary storage: keccak256(abi.encodePacked(validatorID, domain)) → ReputationRecord.
    mapping(bytes32 => ReputationRecord) private _reputation;

    /// @notice Emitted when a validator's reputation score is updated.
    /// @param key         keccak256(abi.encodePacked(validatorID, domain)).
    /// @param validatorID The validator whose reputation changed.
    /// @param domain      The domain in which the change occurred.
    /// @param oldScore    The score before the update (scaled by 1e6).
    /// @param newScore    The score after the update (scaled by 1e6).
    /// @param epoch       Block number at the time of the update.
    event ReputationUpdated(
        bytes32 indexed key,
        string          validatorID,
        string          domain,
        uint64          oldScore,
        uint64          newScore,
        uint64  indexed epoch
    );

    /// @notice Update or create the reputation record for (validatorID, domain).
    ///         Emits ReputationUpdated with the old and new scores.
    /// @param record The new reputation state to store.
    function updateReputation(ReputationRecord calldata record) external {
        bytes32 key = keccak256(abi.encodePacked(record.validatorID, record.domain));
        uint64 oldScore = _reputation[key].score;
        _reputation[key] = record;
        emit ReputationUpdated(
            key,
            record.validatorID,
            record.domain,
            oldScore,
            record.score,
            record.epoch
        );
    }

    /// @notice Returns the reputation record for (validatorID, domain).
    /// @param validatorID The public key hash of the validator.
    /// @param domain      The domain to query.
    /// @return The stored ReputationRecord. All fields are zero if not found.
    function getReputation(string calldata validatorID, string calldata domain)
        external
        view
        returns (ReputationRecord memory)
    {
        return _reputation[keccak256(abi.encodePacked(validatorID, domain))];
    }

    /// @notice Returns the current score for (validatorID, domain).
    ///         Convenience function for callers that only need the score.
    /// @param validatorID The public key hash of the validator.
    /// @param domain      The domain to query.
    /// @return score The reputation score scaled by 1e6. 0 if not found.
    function getScore(string calldata validatorID, string calldata domain)
        external
        view
        returns (uint64 score)
    {
        return _reputation[keccak256(abi.encodePacked(validatorID, domain))].score;
    }
}
