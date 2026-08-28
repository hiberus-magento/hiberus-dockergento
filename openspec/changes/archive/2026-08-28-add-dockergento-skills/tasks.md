# Tasks

## The skills

- [x] `skills/dockergento-environment/SKILL.md`
- [x] `skills/dockergento-database/SKILL.md`
- [x] `skills/dockergento-debugging/SKILL.md`
- [x] `skills/dockergento-agents/SKILL.md`
- [x] Each one short, with the JSON contract and the exit codes an agent needs

## The check

- [x] `tests/unit/skills_test.sh`: commands, options, services and the `hm bash` rule
- [x] Run it against the bundled skills, failing on anything undeclared

## Installing them

- [x] A local repository entry in `data/ai-repositories.json`
- [x] `hm ai-pull` installs a local repository without downloading it
- [x] Integration test: the bundled skills are installed and tracked

## Documentation

- [x] `docs/skills.md`: what they are, where they live, why here
- [x] Changelog and backlog note
