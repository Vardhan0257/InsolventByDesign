package relay

import (
	"math/big"
	"os"
	"path/filepath"
	"testing"
)

// TestParseRelayFile_ValidData verifies correct parsing of well-formed relay data.
//
// Purpose: JSON schema correctness - REQUIRED by blueprint
//
// This test ensures:
// - Slot extraction works
// - Value conversion to big.Int works
// - Builder pubkey is preserved
func TestParseRelayFile_ValidData(t *testing.T) {
	// Create temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_valid.json")

	validJSON := `[
		{
			"slot": "1000",
			"parent_hash": "0xabc",
			"block_hash": "0xdef",
			"builder_pubkey": "0xbuilder1",
			"proposer_pubkey": "0xproposer1",
			"proposer_fee_recipient": "0xfee1",
			"gas_limit": "30000000",
			"gas_used": "29000000",
			"value": "123456789012345678",
			"block_number": "100"
		},
		{
			"slot": "1001",
			"parent_hash": "0xghi",
			"block_hash": "0xjkl",
			"builder_pubkey": "0xbuilder2",
			"proposer_pubkey": "0xproposer2",
			"proposer_fee_recipient": "0xfee2",
			"gas_limit": "30000000",
			"gas_used": "28000000",
			"value": "987654321098765432",
			"block_number": "101"
		}
	]`

	err := os.WriteFile(testFile, []byte(validJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Parse
	bribes, err := ParseRelayFile(testFile)
	if err != nil {
		t.Fatalf("ParseRelayFile failed: %v", err)
	}

	// Verify count
	if len(bribes) != 2 {
		t.Errorf("Expected 2 bribes, got %d", len(bribes))
	}

	// Verify first bribe
	if bribes[0].Slot != 1000 {
		t.Errorf("Expected slot 1000, got %d", bribes[0].Slot)
	}
	expectedValue1 := big.NewInt(123456789012345678)
	if bribes[0].ValueWei.Cmp(expectedValue1) != 0 {
		t.Errorf("Expected value %s, got %s", expectedValue1.String(), bribes[0].ValueWei.String())
	}
	if bribes[0].BuilderPubkey != "0xbuilder1" {
		t.Errorf("Expected builder 0xbuilder1, got %s", bribes[0].BuilderPubkey)
	}

	// Verify second bribe
	if bribes[1].Slot != 1001 {
		t.Errorf("Expected slot 1001, got %d", bribes[1].Slot)
	}
	expectedValue2 := big.NewInt(987654321098765432)
	if bribes[1].ValueWei.Cmp(expectedValue2) != 0 {
		t.Errorf("Expected value %s, got %s", expectedValue2.String(), bribes[1].ValueWei.String())
	}
}

// TestParseRelayFile_BigIntPrecision verifies NO precision loss.
//
// Purpose: BigInt test - REQUIRED by blueprint
//
// This test uses values that would lose precision in float64:
// - Values larger than 2^53
// - Exact wei values that must be preserved
func TestParseRelayFile_BigIntPrecision(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_bigint.json")

	// Use a value larger than float64 can represent precisely
	// This is ~18.4 ETH in wei, with exact precision
	largeValue := "18446744073709551615" // 2^64 - 1

	validJSON := `[
		{
			"slot": "2000",
			"parent_hash": "0x0",
			"block_hash": "0x0",
			"builder_pubkey": "0xbuilder",
			"proposer_pubkey": "0xproposer",
			"proposer_fee_recipient": "0xfee",
			"gas_limit": "30000000",
			"gas_used": "29000000",
			"value": "` + largeValue + `",
			"block_number": "200"
		}
	]`

	err := os.WriteFile(testFile, []byte(validJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	bribes, err := ParseRelayFile(testFile)
	if err != nil {
		t.Fatalf("ParseRelayFile failed: %v", err)
	}

	// Verify exact value preservation
	expected := new(big.Int)
	expected.SetString(largeValue, 10)

	if bribes[0].ValueWei.Cmp(expected) != 0 {
		t.Errorf("Precision lost! Expected %s, got %s", expected.String(), bribes[0].ValueWei.String())
	}

	// Verify exact string round-trip
	if bribes[0].ValueWei.String() != largeValue {
		t.Errorf("String representation changed! Expected %s, got %s", largeValue, bribes[0].ValueWei.String())
	}
}

// TestParseRelayFile_EmptyFile verifies safe failure on empty files.
//
// Purpose: Empty file test - REQUIRED by blueprint
//
// This ensures the system fails loudly rather than silently accepting bad data.
func TestParseRelayFile_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_empty.json")

	// Create empty file
	err := os.WriteFile(testFile, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Attempt parse - should fail
	_, err = ParseRelayFile(testFile)
	if err == nil {
		t.Error("Expected error for empty file, got nil")
	}

	// Verify error message is clear
	if err != nil && len(err.Error()) < 5 {
		t.Error("Error message too short, should be descriptive")
	}
}

// TestParseRelayFile_CorruptJSON verifies deterministic error on malformed JSON.
//
// Purpose: Corrupt JSON test - REQUIRED by blueprint
//
// This ensures the parser fails predictably, not randomly.
func TestParseRelayFile_CorruptJSON(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_corrupt.json")

	// Various forms of corrupt JSON
	corruptCases := []struct {
		name string
		data string
	}{
		{
			name: "truncated",
			data: `[{"slot": "1000", "value": "123"`,
		},
		{
			name: "invalid_syntax",
			data: `[{"slot": 1000, "value": }]`,
		},
		{
			name: "not_array",
			data: `{"slot": "1000", "value": "123"}`,
		},
	}

	for _, tc := range corruptCases {
		t.Run(tc.name, func(t *testing.T) {
			err := os.WriteFile(testFile, []byte(tc.data), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			_, err = ParseRelayFile(testFile)
			if err == nil {
				t.Errorf("Expected error for corrupt JSON (%s), got nil", tc.name)
			}
		})
	}
}

// TestParseRelayFile_InvalidSlot verifies failure on malformed slot values.
func TestParseRelayFile_InvalidSlot(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_invalid_slot.json")

	invalidJSON := `[
		{
			"slot": "not_a_number",
			"parent_hash": "0x0",
			"block_hash": "0x0",
			"builder_pubkey": "0xbuilder",
			"proposer_pubkey": "0xproposer",
			"proposer_fee_recipient": "0xfee",
			"gas_limit": "30000000",
			"gas_used": "29000000",
			"value": "123456789",
			"block_number": "100"
		}
	]`

	err := os.WriteFile(testFile, []byte(invalidJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err = ParseRelayFile(testFile)
	if err == nil {
		t.Error("Expected error for invalid slot, got nil")
	}
}

// TestParseRelayFile_InvalidValue verifies failure on malformed value strings.
func TestParseRelayFile_InvalidValue(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_invalid_value.json")

	invalidJSON := `[
		{
			"slot": "1000",
			"parent_hash": "0x0",
			"block_hash": "0x0",
			"builder_pubkey": "0xbuilder",
			"proposer_pubkey": "0xproposer",
			"proposer_fee_recipient": "0xfee",
			"gas_limit": "30000000",
			"gas_used": "29000000",
			"value": "not_a_number",
			"block_number": "100"
		}
	]`

	err := os.WriteFile(testFile, []byte(invalidJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err = ParseRelayFile(testFile)
	if err == nil {
		t.Error("Expected error for invalid value, got nil")
	}
}

// TestParseRelayFile_NegativeValue verifies rejection of negative values.
func TestParseRelayFile_NegativeValue(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_negative.json")

	invalidJSON := `[
		{
			"slot": "1000",
			"parent_hash": "0x0",
			"block_hash": "0x0",
			"builder_pubkey": "0xbuilder",
			"proposer_pubkey": "0xproposer",
			"proposer_fee_recipient": "0xfee",
			"gas_limit": "30000000",
			"gas_used": "29000000",
			"value": "-123456789",
			"block_number": "100"
		}
	]`

	err := os.WriteFile(testFile, []byte(invalidJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err = ParseRelayFile(testFile)
	if err == nil {
		t.Error("Expected error for negative value, got nil")
	}
}

// TestParseRelayFile_Ordering verifies deterministic slot ordering.
//
// Purpose: Deterministic behavior - REQUIRED by blueprint
//
// This ensures that bribes are always returned in slot order,
// regardless of input order.
func TestParseRelayFile_Ordering(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_ordering.json")

	// Intentionally out of order
	unorderedJSON := `[
		{
			"slot": "1003",
			"parent_hash": "0x0",
			"block_hash": "0x0",
			"builder_pubkey": "0xbuilder",
			"proposer_pubkey": "0xproposer",
			"proposer_fee_recipient": "0xfee",
			"gas_limit": "30000000",
			"gas_used": "29000000",
			"value": "300",
			"block_number": "103"
		},
		{
			"slot": "1001",
			"parent_hash": "0x0",
			"block_hash": "0x0",
			"builder_pubkey": "0xbuilder",
			"proposer_pubkey": "0xproposer",
			"proposer_fee_recipient": "0xfee",
			"gas_limit": "30000000",
			"gas_used": "29000000",
			"value": "100",
			"block_number": "101"
		},
		{
			"slot": "1002",
			"parent_hash": "0x0",
			"block_hash": "0x0",
			"builder_pubkey": "0xbuilder",
			"proposer_pubkey": "0xproposer",
			"proposer_fee_recipient": "0xfee",
			"gas_limit": "30000000",
			"gas_used": "29000000",
			"value": "200",
			"block_number": "102"
		}
	]`

	err := os.WriteFile(testFile, []byte(unorderedJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	bribes, err := ParseRelayFile(testFile)
	if err != nil {
		t.Fatalf("ParseRelayFile failed: %v", err)
	}

	// Verify strict ascending order
	if len(bribes) != 3 {
		t.Fatalf("Expected 3 bribes, got %d", len(bribes))
	}

	if bribes[0].Slot != 1001 {
		t.Errorf("Expected first slot to be 1001, got %d", bribes[0].Slot)
	}
	if bribes[1].Slot != 1002 {
		t.Errorf("Expected second slot to be 1002, got %d", bribes[1].Slot)
	}
	if bribes[2].Slot != 1003 {
		t.Errorf("Expected third slot to be 1003, got %d", bribes[2].Slot)
	}

	// Verify values match correct slots (not corrupted during sort)
	if bribes[0].ValueWei.String() != "100" {
		t.Errorf("Slot 1001 should have value 100, got %s", bribes[0].ValueWei.String())
	}
	if bribes[1].ValueWei.String() != "200" {
		t.Errorf("Slot 1002 should have value 200, got %s", bribes[1].ValueWei.String())
	}
	if bribes[2].ValueWei.String() != "300" {
		t.Errorf("Slot 1003 should have value 300, got %s", bribes[2].ValueWei.String())
	}
}

// TestParseRelayFile_ZeroValue verifies handling of zero-value bids.
func TestParseRelayFile_ZeroValue(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_zero.json")

	zeroJSON := `[
		{
			"slot": "1000",
			"parent_hash": "0x0",
			"block_hash": "0x0",
			"builder_pubkey": "0xbuilder",
			"proposer_pubkey": "0xproposer",
			"proposer_fee_recipient": "0xfee",
			"gas_limit": "30000000",
			"gas_used": "29000000",
			"value": "0",
			"block_number": "100"
		}
	]`

	err := os.WriteFile(testFile, []byte(zeroJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	bribes, err := ParseRelayFile(testFile)
	if err != nil {
		t.Fatalf("ParseRelayFile failed: %v", err)
	}

	if len(bribes) != 1 {
		t.Fatalf("Expected 1 bribe, got %d", len(bribes))
	}

	if bribes[0].ValueWei.Cmp(big.NewInt(0)) != 0 {
		t.Errorf("Expected zero value, got %s", bribes[0].ValueWei.String())
	}
}

// TestParseRelayDirectory verifies directory-level aggregation.
func TestParseRelayDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple test files
	file1 := filepath.Join(tmpDir, "relay1.json")
	file2 := filepath.Join(tmpDir, "relay2.json")

	json1 := `[{"slot": "1000", "parent_hash": "0x0", "block_hash": "0x0", "builder_pubkey": "0xb1", "proposer_pubkey": "0xp", "proposer_fee_recipient": "0xf", "gas_limit": "30000000", "gas_used": "29000000", "value": "100", "block_number": "100"}]`
	json2 := `[{"slot": "1001", "parent_hash": "0x0", "block_hash": "0x0", "builder_pubkey": "0xb2", "proposer_pubkey": "0xp", "proposer_fee_recipient": "0xf", "gas_limit": "30000000", "gas_used": "29000000", "value": "200", "block_number": "101"}]`

	err := os.WriteFile(file1, []byte(json1), 0644)
	if err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	err = os.WriteFile(file2, []byte(json2), 0644)
	if err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	// Parse directory
	bribes, err := ParseRelayDirectory(tmpDir)
	if err != nil {
		t.Fatalf("ParseRelayDirectory failed: %v", err)
	}

	// Should have combined and sorted both files
	if len(bribes) != 2 {
		t.Errorf("Expected 2 bribes from 2 files, got %d", len(bribes))
	}

	// Verify ordering
	if len(bribes) == 2 && bribes[0].Slot > bribes[1].Slot {
		t.Error("Directory parsing did not maintain global slot order")
	}
}
