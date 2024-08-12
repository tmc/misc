SUGGESTION="${1:-$(ctx-suggest)}"
(echo "fix up all of the git-notes-* scripts"; ctx-exec git log --stat -3; ctx-exec git st; ~/code-to-gpt.sh --tracked-only) |cgpt -s "please refactor the git-notes to reflect the recently fixed ones. output a bash script that will populate all revlevant files completely" -p '#!/' | tee fixes.sh
