# Spec: Browser Actions

## Status

Partially implemented.

Conformance covers extraction into outputs, a multi-operation run with model
and fixed conditions, screenshots, downloads, the replay cache, the secret
check, allowed domains, client certificates, dialogs, and validation. Waiting for input and resuming the same
browser need the REST API and are covered by integration tests. Model prompts,
page interaction internals, and profile locking belong to executor tests.

## Scope

This spec defines the `browser.extract` and `browser.run` action boundary: the
`with` contract, model configuration, outputs, artifacts, downloads, the replay
cache, allowed domains, and human input. It does not define how a model chooses
page elements or the browser runtime's prompts.

## Goal

Workflow authors can drive a website that has no API from a step: act on pages
in natural language, extract structured data into step outputs, check page
state, and pause for a person when the site asks for input.

## Behavior

### Actions

`browser.run` takes `with.do`, a nonempty list of operations run in order in one
browser session. `with.url`, when set, is opened before the first operation.

`browser.extract` takes `with.url`, `with.instruction`, and `with.schema`, and
behaves as `browser.run` with one `extract` operation.

Each operation sets exactly one of:

- `goto`: navigate to a URL.
- `act`: perform one action described in natural language, such as a click
  or typing into one field. The value is an instruction string or an object
  with `instruction` and optional `cache`.
- `extract`: `{instruction, schema}`. The schema must be a JSON Schema with
  `type: object`. A string field with `format: uri` or `format: uri-reference`
  gets the address of the element the model picks, such as a link's target;
  any other string field gets what the model writes, which for a link is its
  element id, such as `0-11`.
- `expect`: a condition that must hold; otherwise the step fails with the
  reason.
- `wait`: `{selector}` waits until the element is visible; `{duration}` pauses.
- `screenshot`: a name; the page is saved as a PNG run artifact.
- `ask`: `{prompt, as, timeout}` waits for a person's answer (see Human input).

A `goto`, or an `act` performed or replayed, ends once the page settles, so the
next operation reads what it brought: a page it loaded waits until its network
is idle, and a page it changed in place until its requests end and it stops
changing for half a second. A page that has not settled after five seconds is
read as it is.

Any operation may set `when`, a condition checked once before the operation;
the operation is skipped unless it holds. Any operation may set `timeout`, a
duration such as `30s`; the default is two minutes. A browser that stops
responding fails the step: an operation is abandoned a few seconds after its
timeout, and a screenshot or page check after 30 seconds.

### Conditions

A condition is either:

- a string: a statement about the page, judged by the model; or
- an object with exactly one of `text` (the page's visible text contains it),
  `selector` (a CSS selector matches a visible element), or `url` (the current
  URL contains it). These fixed checks read the page and make no model request.

A fixed check may set `within`, a duration during which it keeps reading the
page until it holds. A fixed `when` reads the page once unless `within` is set;
a fixed `expect` keeps reading until `within`, or the operation timeout when
`within` is unset. A model-judged condition is asked once. A `selector` check
holds when any matching element is visible.

### Model

A browser step uses the DAG-level `llm` block. `with.llm` replaces it entirely,
with the same shape and providers as the DAG-level block. A browser step with no
model configuration fails validation. When several models are listed, each
model request tries them in order.

Every model request asks for a tool call whose parameters are the response
schema. A reply without a tool call is accepted when its text is valid JSON.
Models that follow tool-call schemas reliably are required for dependable
results.

### Variables and secrets

`with.variables` maps names to values. An `act` instruction references them as
`%name%`, and so does a later act for an `ask.as` name. A reference to any other
name fails validation. An act that references the answer of an `ask` that was
skipped fails the step. The browser receives variable values; requests to the
model carry only the names.

The step fails before starting a browser when an `act`, `extract`, `ask`, or
model-judged `expect` or `when` text contains the resolved value of a secret
declared under `secrets:` that has four or more characters. Values that only
come from `env:` are not secrets and are not checked.

Declared secret values and `ask` answers of four or more characters are masked
in text sent to the model, including the page text, in the step log, and in
the timeline. Other variable values are not masked.

A model request carries the instruction or judged statement, the page's
elements with their roles and visible text, and the extract schema or the
names of the available variables. It never carries variable values, text the
browser typed into fields, or screenshots.

### Outputs

The top-level properties each `extract` schema lists become step outputs,
readable as `${steps.<id>.outputs.<name>}` and known when the DAG loads. Two
extract operations in one step that list the same property fail validation.
Only listed properties are published; extracted fields a schema does not list
are dropped. When the step succeeds with outputs, stdout is one JSON object of
those outputs. Operation progress is written to stderr.

### Browser options

`with.browser` accepts `headless` (default `true`), `executable` (otherwise
`CHROME_PATH` or an installed Chrome), `viewport` `{width, height}`, `proxy`
(unauthenticated), `allowed_domains`, `screenshots`, `profile`, and
`client_certificates`.

The host configuration `browser.sandbox: false`, or the environment variable
`DAGU_BROWSER_SANDBOX=false`, turns off Chromium's sandbox for every browser
the host starts, for hosts where the sandbox cannot start. The sandbox is on
by default, and a DAG cannot change it. With the sandbox on, a step fails
before starting a browser where the browser runtime would turn the sandbox off
anyway: when `CI` is set, or when Dagu runs as root on Linux.

### Client certificates

`client_certificates` lists TLS client certificates and the hosts each is
presented to, so a site that requires one opens without the operating system
asking for the certificate's password. An entry names `hosts`, either
`pfx_file` with `passphrase` or `passphrase_file`, or `cert_file` and an
unencrypted `key_file` in PEM, and optionally `ca_file`, PEM certificates
trusted for those hosts' servers besides the system's. A host is a host name
or IP address, `*.example.com` for the subdomains of `example.com`, or `*` for
every host. The step's relative files resolve against its working directory.

The host configuration `browser.client_certificates` takes the same entries,
with `passphrase_file` for a passphrase, and applies to every browser the host
starts, including `dagu browser session` browsers. A step's entries come
first, and the first entry whose hosts match a request is used. An entry with
no hosts, a malformed host, both or neither of the file forms, or a passphrase
for a PEM key fails validation; host configuration entries fail at startup.

The certificates are read before the browser starts: an unreadable file, a
wrong passphrase, or a key that does not match its certificate fails the step
then, naming the entry without its passphrase. A certificate outside its
validity period is a warning. A passphrase written in a step is masked and
kept from the model like a secret.

Dagu presents the certificates through a proxy process that runs as long as
the browser. The browser sends it the HTTPS requests to the entries' hosts, and
the proxy reaches those hosts the way the device does: through `proxy` when it
is set, otherwise through the operating system's proxy settings (the proxy
auto-config, automatic detection on Windows, or the manual proxies; the proxy
environment variables on Linux). The browser sends every other request there
itself. With certificates, `proxy` takes only `[scheme://]host[:port]` with
scheme `http`, `https`, or `socks5`. A proxy that asks for authentication
cannot carry requests to certificate hosts, and the proxy, not the browser,
verifies those hosts' server certificates.

While a browser runs, any process on the host, including another operating
system user's, can use its certificates: through the browser's DevTools
address, which already lets a local process drive the browser, or through the
proxy, which listens on the loopback interface. Present certificates only on
hosts whose users are trusted with them.

While the browser presents certificates, it also guards its other HTTPS
requests: the proxy carries them too, presenting no certificate, so the
browser never asks the operating system for one, which would wait on a
system prompt nobody answers. The host configuration
`browser.guard_client_certificates` (or `DAGU_BROWSER_GUARD_CLIENT_CERTIFICATES`)
turns the guard off, or on for every browser even without certificates. When
the device's proxy asks the certificate proxy for credentials it cannot give,
such as a Windows sign-in, the guard stops and the browser reaches those sites
itself; the step's log says so. The guard trusts the system's CA certificates
and every entry's `ca_file`.

A page that does not load because of a client certificate fails the
operation that opened it: the error names what the proxy saw, such as the
site rejecting the certificate, or says the site asks for a certificate and
none is configured for it. That includes a site that answers with an error
page, such as nginx's 400 or IIS's 403.7, rather than refusing the
connection. A site that asks for a certificate and opens without one is not a
failure. An operation that times out names the connections the proxy still
waits on and how far each got. The step's log records each host the proxy
presented a certificate to, each site that asked for one, and each failure.

### Allowed domains

The browser runtime applies `allowed_domains` to the page's HTTP(S) requests,
including scripts, images, and API calls, so a site's CDN and sign-in hosts
must be listed. WebSocket connections are not covered, and the runtime's check
can be bypassed. `example.com` matches only that host; `*.example.com` matches
its subdomains but not `example.com`. An entry with a scheme, port, or path, a
`*` other than a leading `*.`, or fewer than two labels fails validation.

Dagu itself checks only the page URL: a `goto` or `with.url` outside the list
fails before navigating, and after the start URL and after every operation the
step fails when a redirect or an action left the allowed domains. Pages without a network
host, such as `about:blank`, are not checked.

Requests the runtime blocks are counted per host. When the start URL or an
operation causes blocked requests, the timeline and the step log show a line
after it such as `allowed_domains blocked 14 requests: cdn.example.com (12),
sso.example.com (2)`, naming up to ten hosts, most blocked first. A failed
step's error ends with the same summary of every request blocked during the
attempt, since a page missing a script or a sign-in redirect usually fails a
later operation. A request blocked after its operation returns is counted with
the next one. When counting stops, for example because the browser's DevTools
connection closes, the step log warns once.

### Artifacts

A DAG with a browser action enables artifact storage unless it sets
`artifacts.enabled: false`. Files are written under `browser/<step id>/` in the
run's artifacts directory and are not masked.

| `screenshots` | Automatic screenshots |
| --- | --- |
| `on_failure` (default) | When the step fails. |
| `final` | When the step fails, and at the end of a successful step. |
| `each` | After every operation, plus the `final` ones. |
| `never` | None. |

With artifacts disabled, no automatic screenshots are saved, a `screenshot`
operation fails, and the browser refuses downloads.

### Downloads

Files the page downloads are saved under `browser/<step id>/downloads/` with
the name the site suggests, made unique within the directory. Only `act` and
`goto` operations start downloads. Once one has run, the step waits for running
downloads after every operation, and before it ends or pauses for an `ask` it
also waits a few seconds for a download to begin. A download may run for the
longest timeout of the acts and gotos run so far. A canceled download, or one
still running at that timeout, fails the step.

### Dialogs

The step accepts every JavaScript dialog a page opens, so a dialog never
blocks the page: `alert`, `confirm`, and `beforeunload` are accepted, and a
`prompt` is answered with its default text. Each accepted dialog appears in
the timeline and the step log after the operation that opened it.

### Replay cache

With `with.cache` true (the default), a successful `act` records the actions it
performed. A later run of the same step on the same host replays them without a
model request when the operation position, instruction, and page URL without
query or fragment match. When a replay fails, the step asks the model again.
If the step succeeds, the model's new actions replace the recording; an act
that records no actions, such as one whose click loaded a new document before
it reported back, removes the recording instead, unless another run of the step
replaced it first. A replay fails when a recorded element is gone, is on the
page but not visible, such as a field in a closed dialog, or is no longer the
element the act recorded: the element at the recorded location has another
accessibility role or name, or a select there does not offer the recorded
option. When other elements on the page shared the recorded element's role and
name, such as a Delete button in each row of a list, the text of its row must
also match. Without such a row, the replay fails when the page has more
elements of that role and name than when the act recorded it. An element
without a name is told apart by its role only.
`act.cache: false` disables the cache for one operation.

Runs of a step share its recordings, and each `act` reads them when it runs. A
recording that another run of the step replaced or removed meanwhile, such as
an earlier `foreach` item waiting for the same profile, is not replayed.

The cache covers `act` only. `extract` and model-judged conditions make model
requests on every run.

`dagu browser cache clear <dag>` removes the recorded actions of every step of
the DAG, or of one step with `--step <id>`, and reports the steps it removed.
`DELETE /dags/{fileName}/browser-cache[?step=<id>]` does the same and returns
the removed steps. Removing all of a DAG's history with `dagu rm --history`
also clears its cache; removing only older runs keeps it. Each clears the cache
on its own host only. After a clear, the next run of the step makes a model
request and records again.

`dagu browser session export` writes the recordings of the acts a browser
session ran for a step that has not run yet (Spec 079).

### Profiles

`browser.profile` names a persistent browser profile kept on the executing
host. Cookies and storage survive across runs on that host. Runs that use the
same profile run one at a time; a run fails immediately when another run
waiting for input, or a browser session (Spec 079), holds the profile.

### Human input

An `ask` operation puts the step in `Waiting` with a pending question, keeps the
browser open, and ends the step execution. Answering the question from the Web
UI or REST API resumes the step in the same browser, on the host that holds it,
at the operation after the `ask`. The answer is available to later `act`
instructions as `%<as>%`. Rejecting the question fails the step. The browser
stays open for `ask.timeout` (default one hour); an answer after that fails the
step. The pending question carries that deadline as `expiresAt`.

Answers are stored in the run's history like other human input.

On Windows, a step with an `ask` operation fails before starting a browser,
because the browser cannot outlive the step process there.

## Errors

A missing `with.do`, `with.url` for `browser.extract`, `with.instruction`, or
`with.schema` fails validation with a diagnostic naming the field. An operation
or condition that sets zero or several keys fails validation. A step fails when
an `act` does not complete, an `expect` does not hold, a selector does not
appear before the timeout, a download does not finish, the page leaves the
allowed domains, the browser stops responding, or the browser cannot be
started. When an `act` fails because its model chose no element, the error
names the model, since some models give that answer for every request. An
`act` whose model picks an option that the select does not offer does not
complete. A page that does not load because of a client certificate fails the
operation that opened it, as described under Client certificates.

Loading a DAG warns, without failing, about each `extract` string field whose
name ends in `url`, `uri`, `link`, or `href`, singular or plural, and that has
neither address format, naming the operation and the field's path, such as
`do[0]: extract field "listings[].link"`.

The browser can lose its connection to the page before an `act` or a replay
reports back, as when a click or a chosen option loads a new page. The step
then checks the page instead of acting blindly, which could submit a form
twice: a new document means the action took effect, and the step goes on; the
same document means it did not, and the act runs once more. Losing the page
again, or a page that cannot be read, fails the step. So does losing the page
to a frame that loads a new document while the page keeps its own: the action
may have caused that load, so acting again could repeat it.

## Examples

```yaml
secrets:
  - name: SHOP_COUPON
    provider: env
    key: SHOP_COUPON

llm:
  provider: anthropic
  model: claude-sonnet-5

steps:
  - id: checkout
    action: browser.run
    with:
      url: https://shop.example.com/cart
      browser:
        # Every host the site loads from, including CDNs.
        allowed_domains: ["*.example.com"]
      variables:
        coupon: ${SHOP_COUPON}
      do:
        - act: Type %coupon% into the coupon field
        - act: Click the Apply button
        - act: Click the Place order button
        - expect: {text: Order confirmed}
        - extract:
            instruction: The order number
            schema:
              type: object
              properties:
                order_number: { type: string }

  - id: record
    depends: checkout
    run: echo "${steps.checkout.outputs.order_number}"
```
