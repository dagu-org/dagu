# Spec: Data Convert and Pick Actions

## Status

Partially implemented.

Conformance covers JSON-to-YAML conversion, raw selection, a text round trip,
and representative errors. Delimited formats and option permutations belong in
data executor unit tests.

## Scope

This spec defines the action boundary for `data.convert` and `data.pick`.
It does not define every serializer option or the selection language itself.

## Goal

Workflow authors can convert data and select values without an external tool.

## Behavior

`with.from` and `with.to` accept `json`, `yaml`, `csv`, `tsv`, and `text`.
`data.convert` decodes its input and serializes the resulting value in the
requested output format. JSON string input must be decoded as JSON.

For `text`, strings are preserved, null becomes an empty string, and other values
are represented as JSON text. Text output adds no newline.

`data.pick` evaluates `with.select` against the decoded input. With `with.raw`,
a string result is written without quotes and followed by a newline. A null
selection succeeds and writes exactly one newline.

## Errors

A missing input format or selection expression fails validation with a diagnostic
identifying the field. Malformed JSON input fails execution with a JSON decode
diagnostic.

## Examples

```yaml
steps:
  - action: data.convert
    with:
      from: json
      to: yaml
      data: '{"name":"alice","age":30}'
```
