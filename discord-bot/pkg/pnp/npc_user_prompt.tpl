"""
- OLD TRANSCRIPTS -
{{ range $index, $transcript := .OldTranscripts -}}
{{ $index }}:
{{ $transcript }}
{{ end }}

- CURRENT TRANSCRIPT -
{{ .CurrentTranscript }}
"""