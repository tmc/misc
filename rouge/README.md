# Rouge

Rouge is a CLI tool to compute ROUGE-L similarity scores between a reference and a candidate text file. It identifies the Longest Common Subsequence (LCS) and highlights differences.

## Usage

```
rouge [flags] <reference_file> <candidate_file>
```

-   `-t threshold`: Minimum ROUGE-L F1-score threshold (0.0-1.0). Exit 1 if below.
-   `-json`: Output results in JSON format to stdout.
-   `-p mode`: Print file content with highlighting ('ref' or 'cand', default 'cand').
    -   `cand`: Print candidate file. Green=LCS, Red=Top Extra Blocks.
    -   `ref`: Print reference file. Green=LCS, Blue=Top Missing Blocks.
-   `-n N`: Number of top-length LCSs to find/show (Currently finds only 1, default: 1). Flag kept for potential future extension.
-   `-e N`: Highlight and list the top N longest contiguous blocks in the **candidate** not part of the primary LCS (default: 0).
-   `-m N`: Highlight and list the top N longest contiguous blocks in the **reference** not part of the primary LCS (default: 0).

**Note:**
*   The standard Longest Common Subsequence is used for ROUGE calculation and diff block identification.
*   When using `-p`, scores and block lists go to stderr, highlighted text goes to stdout.

## Examples

1.  Compare `testdata/b.txt` (cand) against `testdata/a.txt` (ref), show highlighted `b.txt`:
    ```bash
    ./rouge testdata/a.txt testdata/b.txt
    ```

2.  Compare, show top 1 extra block (red) in `b.txt`, show top 1 missing block (blue) when viewing `a.txt`:
    ```bash
    # View candidate b.txt (red highlights)
    ./rouge -e 1 -m 1 testdata/a.txt testdata/b.txt

    # View reference a.txt (blue highlights)
    ./rouge -e 1 -m 1 -p ref testdata/a.txt testdata/b.txt
    ```

3.  Compare with JSON output:
    ```bash
    ./rouge -json -e 3 -m 3 testdata/a.txt testdata/b.txt
    ```

## Output Format

-   **Text (Default):**
    -   *Stderr:* ROUGE-L scores, the LCS string, lists of Top-E Extra and Top-M Missing blocks.
    -   *Stdout:* Highlighted text (`-p` selection) using ANSI colors:
        -   Green: Word is part of the LCS.
        -   Red: Word is part of a top "Extra" block (candidate-only).
        -   Blue: Word is part of a top "Missing" block (reference-only).
        -   Default: Word not in LCS and not in a top block.
-   **JSON (`-json`):** Prints a JSON object to stdout containing ROUGE-L scores, the LCS string, and the lists of top extra/missing blocks.

## Exit Codes

-   `0`: Success.
-   `1`: F1 score below threshold, or file error.
-   `2`: Usage error (invalid flags/arguments).

