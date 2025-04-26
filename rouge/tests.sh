#!/usr/bin/env bash
set -euo pipefail

# Build / vet once
echo "Building rouge tool..."
go vet .
go build .
echo "Build complete."

# --- Create Test Files ---
TESTDATA_DIR="testdata"
echo "Creating test files in ${TESTDATA_DIR}/..."
mkdir -p "${TESTDATA_DIR}"
# Basic Cases
echo "hi there buddy how ya doin?" > "${TESTDATA_DIR}/a.txt"
echo "hi there friend how are ya doin?" > "${TESTDATA_DIR}/b.txt"
echo "the quick brown fox jumps over the lazy dog" > "${TESTDATA_DIR}/c.txt"
echo "the slow brown cat jumps over a lazy dog" > "${TESTDATA_DIR}/d.txt"

# Multiple LCS / More Complex Diffs
echo "Line one common." > "${TESTDATA_DIR}/e_ref.txt"
echo "Line two differs here significantly." >> "${TESTDATA_DIR}/e_ref.txt"
echo "Line three common again." >> "${TESTDATA_DIR}/e_ref.txt"
echo "Line four is unique to reference." >> "${TESTDATA_DIR}/e_ref.txt"
echo "Line five common." >> "${TESTDATA_DIR}/e_ref.txt"

echo "Line one common." > "${TESTDATA_DIR}/f_cand.txt"
echo "Line two varies somewhat differently." >> "${TESTDATA_DIR}/f_cand.txt"
echo "Line three common again." >> "${TESTDATA_DIR}/f_cand.txt"
echo "Line four is added in candidate." >> "${TESTDATA_DIR}/f_cand.txt"
echo "Line five common." >> "${TESTDATA_DIR}/f_cand.txt"

# Files designed for multiple max-length LCS (though only 1 found now)
echo "A B C D E F" > "${TESTDATA_DIR}/g_ref.txt"
echo "A X C D Y F" > "${TESTDATA_DIR}/h_cand.txt" # LCS: "A C D F" (length 4)
echo "A Z C D W F" > "${TESTDATA_DIR}/i_cand.txt" # LCS: "A C D F" (length 4) - same LCS text, different path

# Edge Cases
echo "Identical line one." > "${TESTDATA_DIR}/identical1.txt"
echo "Identical line one." > "${TESTDATA_DIR}/identical2.txt"
touch "${TESTDATA_DIR}/empty.txt"
echo "No overlap here" > "${TESTDATA_DIR}/no_overlap1.txt"
echo "Completely different text" > "${TESTDATA_DIR}/no_overlap2.txt"

# Whitespace and Punctuation
echo "First sentence.  Second sentence; with punctuation!
New line,	with tabs	and spaces." > "${TESTDATA_DIR}/punct_ref.txt"
echo "First sentence. Second NEW sentence; different punctuation?
New line, with tabs... and spaces." > "${TESTDATA_DIR}/punct_cand.txt"

echo "Test files created."

# --- Helper Function for Readability ---
run_test() {
    echo -e "\n--- Test: $@ ---"
    # Prepend testdata dir to file arguments
    args=()
    ref_file=""
    cand_file=""
    has_files=false
    for arg in "$@"; do
        if [[ "$arg" == *.txt && -f "${TESTDATA_DIR}/${arg}" ]]; then
            args+=("${TESTDATA_DIR}/${arg}")
            if [ "$has_files" = false ]; then
                ref_file="${TESTDATA_DIR}/${arg}"
                has_files=true
            else
                cand_file="${TESTDATA_DIR}/${arg}"
            fi
        elif [[ "$arg" == empty.txt ]]; then # Handle empty.txt specially
             args+=("${TESTDATA_DIR}/${arg}")
             if [ "$has_files" = false ]; then
                ref_file="${TESTDATA_DIR}/${arg}"
                has_files=true
            else
                cand_file="${TESTDATA_DIR}/${arg}"
            fi
        else
            args+=("$arg")
        fi
    done

    echo "Command: ./rouge ${args[@]}"
    # Print file contents if they exist
    if [ -n "$ref_file" ] && [ -f "$ref_file" ]; then
        echo "Reference (${ref_file}): $(cat "$ref_file")"
    elif [ -n "$ref_file" ]; then
         echo "Reference (${ref_file}): (File does not exist or is empty)" # Handle empty.txt case display
    fi
     if [ -n "$cand_file" ] && [ -f "$cand_file" ]; then
        echo "Candidate (${cand_file}): $(cat "$cand_file")"
     elif [ -n "$cand_file" ]; then
          echo "Candidate (${cand_file}): (File does not exist or is empty)" # Handle empty.txt case display
     fi
    echo "--- Output ---"
    # Use || true to prevent script exit if threshold fails (which is expected in some tests)
    ./rouge "${args[@]}" || true
    echo "----------------------------------------"

}

# --- Run Tests ---

echo -e "\n\n=== BASIC COMPARISONS (a.txt vs b.txt) ==="
run_test a.txt b.txt                                    # Default: highlight cand, n=1, e=0, m=0
run_test -p ref a.txt b.txt                             # Highlight ref
run_test -e 1 a.txt b.txt                               # Highlight cand, show top 1 extra (red) -> 'friend' or 'are'
run_test -e 2 a.txt b.txt                               # Highlight cand, show top 2 extra (red) -> 'friend', 'are'
run_test -p ref -m 1 a.txt b.txt                        # Highlight ref, show top 1 missing (blue) -> 'buddy'

echo -e "\n\n=== MORE COMPLEX DIFFS (e_ref.txt vs f_cand.txt) ==="
run_test -n 2 -e 2 -m 2 e_ref.txt f_cand.txt            # Show more LCS, Extra, Missing blocks in Candidate
run_test -p ref -n 2 -e 2 -m 2 e_ref.txt f_cand.txt     # Show more LCS, Extra, Missing blocks in Reference

echo -e "\n\n=== MULTIPLE LCS EXAMPLES (g_ref.txt vs h_cand.txt / i_cand.txt) ==="
run_test -n 3 g_ref.txt h_cand.txt                      # Try to show multiple LCS (will show 1 of N requested)
run_test -n 3 g_ref.txt i_cand.txt                      # Compare ref against different candidate with same max LCS length
run_test -n 1 -e 1 -m 1 g_ref.txt h_cand.txt            # Basic diff highlighting for this case

echo -e "\n\n=== THRESHOLD TESTS (c.txt vs d.txt) ==="
run_test -t 0.6 c.txt d.txt                             # Threshold Pass (F1 expected ~0.6316)
run_test -t 0.7 c.txt d.txt                             # Threshold Fail (F1 expected ~0.6316)

echo -e "\n\n=== JSON OUTPUT ==="
run_test -json -n 1 -e 2 -m 2 e_ref.txt f_cand.txt      # Complex case with JSON output (using n=1 for simplicity)

echo -e "\n\n=== EDGE CASES ==="
run_test identical1.txt identical2.txt                  # Expect F1=1.0, all green
run_test a.txt empty.txt                                # Expect F1=0.0
run_test empty.txt b.txt                                # Expect F1=0.0
run_test -p ref empty.txt b.txt                         # Expect F1=0.0
run_test empty.txt empty.txt                            # Expect F1=1.0
run_test no_overlap1.txt no_overlap2.txt               # Expect F1=0.0

echo -e "\n\n=== WHITESPACE & PUNCTUATION (punct_ref.txt vs punct_cand.txt) ==="
run_test -e 2 -m 2 punct_ref.txt punct_cand.txt         # Check highlighting with varied spacing/punctuation
run_test -p ref -e 2 -m 2 punct_ref.txt punct_cand.txt  # Check highlighting on reference side


# --- Cleanup ---
echo -e "\n--- Cleaning up test files ---"
rm -f ./rouge
rm -rf "${TESTDATA_DIR}" # Remove the whole testdata directory
echo "Cleanup complete."

echo -e "\nTests finished."

