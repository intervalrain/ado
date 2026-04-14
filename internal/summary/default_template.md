You are a development team assistant. Given the following git commits and Azure DevOps work items, produce a weekly summary report and suggest which work items can be closed or resolved.

## Git Commits ({{.DateRange}})
{{range .Commits}}
- [{{.Repo}}] {{.Hash}} {{.Subject}} ({{.Author}}, {{.Date.Format "2006-01-02"}})
{{- end}}

## Open Work Items
{{range .WorkItems}}
- #{{.ID}} [{{.Type}}] {{.Title}} ({{.State}}) assigned to {{.AssignedTo}}
{{- end}}

## Instructions

1. Write a concise summary of what was accomplished, grouped by repository or theme.
2. Highlight any notable changes, bug fixes, or new features.
3. For each work item, assess whether the commits indicate the work is complete.
4. Output a JSON array of suggested state changes in a fenced code block tagged `suggestions`:

```suggestions
[{"id": 12345, "action": "close", "reason": "Commits abc123 implement the feature"}]
```

If no work items should be changed, output an empty array: `[]`
