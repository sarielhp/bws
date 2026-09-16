set -eu
git_safe() {
    /usr/bin/git -c core.hooksPath=/dev/null -c core.fsmonitor=false \
        -c commit.gpgsign=false -c user.name='bws agent' \
        -c user.email=bws-agent@localhost "$@"
}
branch="refs/heads/$1"
test "$(git_safe symbolic-ref HEAD)" = "$branch"
dirty=$(git_safe status --porcelain)
if test -n "$dirty"; then
    git_safe add -A >&2
    git_safe reset -q HEAD -- .bws .bws.jsonc '.env*' >&2
    if ! git_safe diff --cached --quiet; then
        git_safe commit -m "bws(agent): changes from session on $1" >&2
    fi
fi
git_safe bundle create - "$branch"
