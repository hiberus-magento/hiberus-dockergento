# verify

Runs whatever code checks this project has installed, and says whether they passed.

```bash
hm verify              # the fast ones
hm verify --changed    # only what differs from the base branch
hm verify --all        # plus unit tests and DI compilation
hm verify --json       # for a script, an agent, or CI
```

## Why it looks at what is there

The tools are not the same in any two projects. Of the fourteen on the machine this was built for:

| Tool | Projects that had it |
|---|---|
| PHPUnit | 10 |
| PHP-CS-Fixer | 9 |
| PHPStan | 6 |
| Magento coding standard | 5 |
| None at all | 3 |

A fixed list of checks would fail almost everywhere. So each check knows how to tell whether it
applies, and **what is missing is reported as skipped, never as a failure**:

```console
$ hm verify

Verifying my-project (everything)

  ✓  syntax
  ✗  coding-standard     14 problem(s)
  –  static-analysis     phpstan/phpstan is not installed
  ✓  formatting
  –  unit-tests          slow: run with --all
  –  di-compile          slow: run with --all
```

"This project has no PHPStan" and "PHPStan failed" lead to opposite actions, and the output keeps
them apart.

## Syntax is always checked

`php -l` needs nothing installed, and it catches what hurts most: a file that takes the whole site
down. In a project with no tooling at all, `hm verify` still does something useful.

## Fast by default

`php -l`, PHPCS, PHPStan and PHP-CS-Fixer take seconds. Unit tests take minutes and DI compilation
can take much longer. A command that takes five minutes does not get run after every change, and a
command that does not get run is worth nothing — so the slow ones need `--all`.

## `--changed` compares against the branch, not the commit

When closing a task, what matters is everything the branch changed, not what was in the last
commit. If there is no base branch to compare against, everything is checked and the report says
so, rather than checking a part while implying it is the whole.

## It never fixes anything

Deliberately. A command that repairs what it finds makes changes nobody has read, and with agents
in the loop that is exactly what you do not want. Fix with the tools' own commands — `php-cs-fixer
fix`, `phpcbf` — after looking at what they propose.

## Exit codes

`0` when nothing failed, non-zero when something did. Skipped is not failed, so a project without
tools exits `0`. That makes it usable as the last step of an agent's task or as a CI stage.

In JSON, `.data.checks[]` carries `name`, `status`, `problems` and the tool's own `output`.
