package campaign

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func verifyInputs(binary, script, binaryHash, scriptHash string, assets map[string]string) error {
	for path, want := range assets {
		hash, e := fileHash(path)
		if e != nil || hash != want {
			return fmt.Errorf("declared asset changed: %s", path)
		}
	}
	for path, want := range map[string]string{binary: binaryHash, script: scriptHash} {
		hash, e := fileHash(path)
		if e != nil || hash != want {
			return fmt.Errorf("binary or script changed")
		}
	}
	return nil
}
func verifyAttempts(m *Manifest, directory string) error {
	if e := verifyArtifactInventory(m, directory); e != nil {
		return e
	}
	for i := range m.Candidates {
		cell := &m.Candidates[i]
		for j := range cell.Attempts {
			a := &cell.Attempts[j]
			path := filepath.Join(directory, a.Directory, "attempt.json")
			b, e := readLimited(path, 16384)
			interrupted := a.Error == "interrupted before completion" && a.ExitCode == -1 && a.Value == nil && a.SummaryHash == "" && !a.Passed
			if e != nil {
				if os.IsNotExist(e) && interrupted {
					continue
				}
				return fmt.Errorf("completed attempt receipt missing: %w", e)
			}
			var receipt struct {
				Identity, ID string
				Parameters   map[string]string
				Attempt      Attempt
			}
			if e = json.Unmarshal(b, &receipt); e != nil {
				return fmt.Errorf("invalid attempt receipt: %w", e)
			}
			if receipt.Identity != m.Identity || receipt.ID != cell.ID || cellID(receipt.Parameters) != cell.ID {
				return fmt.Errorf("attempt receipt identity differs")
			}
			if interrupted {
				if receipt.Attempt.Number != a.Number || receipt.Attempt.Directory != a.Directory || !receipt.Attempt.StartedAt.Equal(a.StartedAt) {
					return fmt.Errorf("interrupted attempt identity differs")
				}
				*a = receipt.Attempt
			} else {
				saved, _ := json.Marshal(receipt.Attempt)
				checkpoint, _ := json.Marshal(a)
				if string(saved) != string(checkpoint) {
					return fmt.Errorf("checkpoint attempt differs from retained receipt")
				}
			}
			if a.SummaryHash != "" {
				hash, e := fileHash(filepath.Join(directory, a.Directory, "summary.json"))
				if e != nil || hash != a.SummaryHash {
					return fmt.Errorf("checkpoint summary missing or changed")
				}
			}
		}
	}
	return nil
}

func verifyArtifactInventory(m *Manifest, directory string) error {
	cells := map[string]map[string]bool{}
	for _, cell := range m.Candidates {
		attempts := map[string]bool{}
		for _, a := range cell.Attempts {
			attempts[filepath.Base(a.Directory)] = true
		}
		cells[cell.ID] = attempts
	}
	entries, e := os.ReadDir(directory)
	if e != nil {
		return e
	}
	for _, entry := range entries {
		// Only our generated 24-hex-character identity namespace is owned here.
		if !entry.IsDir() || len(entry.Name()) != 24 {
			continue
		}
		if _, e := hex.DecodeString(entry.Name()); e != nil {
			continue
		}
		attempts, known := cells[entry.Name()]
		if !known {
			return fmt.Errorf("unreferenced candidate artifacts; checkpoint is incomplete")
		}
		records, e := os.ReadDir(filepath.Join(directory, entry.Name()))
		if e != nil {
			return e
		}
		for _, record := range records {
			if !record.IsDir() || !strings.HasPrefix(record.Name(), "attempt-") {
				continue
			}
			if !attempts[record.Name()] {
				return fmt.Errorf("unreferenced attempt artifacts; checkpoint is incomplete")
			}
		}
	}
	return nil
}
