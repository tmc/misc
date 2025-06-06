(
  export BASH_XTRACEFD=3
  PS4=">> pipeline: "
  exec 3> >(sed 's/^+[[:space:]]*/>> pipeline: /' >&2)
  set -x
  git show-ref HEAD
  git status .
  git log --stat -5 . | head -n 500
  git diff --cached --stat . | head -n 1000
  git diff --cached . | head -n 2000
  git diff --stat . | head -n 50
  git diff . | head -n 500
  echo "$@"
) 2>&1 | cat
