# Project Configuration

## Repository Registry

| Repository | Role | Serena Instance | Path |
|---|---|---|---|
| helm-charts | Helm charts | serena | /home/rravi/redhat/trustify-helm-charts/0711/upstream/trustify-helm-charts |

## Jira Configuration

- Project key: TC
- Cloud ID: 2b9e35e3-6bd3-4cec-b838-f4249ee02432
- Feature issue type ID: 10142
- Git Pull Request custom field: N/A
- GitHub Issue custom field: N/A

### Jira Field Defaults

- Default priority: Normal
- fixVersion scope: both
- Prompt for priority: true
- Prompt for fixVersion: true

## Code Intelligence

Tools are prefixed by Serena instance name: `mcp__<instance>__<tool>`.

For example, to search for a symbol in the helm-charts repository:

    mcp__serena__find_symbol(
      name_path_pattern="MyService",
      substring_matching=true,
      include_body=false
    )

### Limitations

No known limitations.

## Bug Configuration

- Bug issue type ID: 10016
- Bug template: docs/bug-template.md
- Bug-to-Task link type: Blocks

## Hierarchy Configuration

- Default epic grouping strategy: by-repository
