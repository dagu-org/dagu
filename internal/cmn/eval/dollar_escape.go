package eval

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
)

type dollarEscapeTokens struct {
	single string
	double string
}

type dollarEscapeKey struct{}

var dollarEscapeSeq uint64

// withDollarEscapes replaces $$ with sentinel tokens so Dagu expansion won't
// treat them as variable prefixes. $$ inside balanced single quotes is preserved.
func withDollarEscapes(ctx context.Context, input string) (context.Context, string) {
	if !strings.Contains(input, "$$") {
		return ctx, input
	}
	if ctx == nil {
		ctx = context.Background()
	}

	spans := singleQuoteSpans(input)
	singleToken := uniqueToken(input, "__DAGU_DOLLAR_ESC_SINGLE__")
	doubleToken := uniqueToken(input, "__DAGU_DOLLAR_ESC_DOUBLE__")

	var b strings.Builder
	b.Grow(len(input))

	spanIdx := 0
	for i := 0; i < len(input); {
		for spanIdx < len(spans) && i > spans[spanIdx].end {
			spanIdx++
		}
		inQuote := spanIdx < len(spans) && i > spans[spanIdx].start && i < spans[spanIdx].end

		if input[i] == '$' && i+1 < len(input) && input[i+1] == '$' {
			if inQuote {
				b.WriteString(doubleToken)
			} else {
				b.WriteString(singleToken)
			}
			i += 2
			continue
		}

		b.WriteByte(input[i])
		i++
	}

	tokens := dollarEscapeTokens{single: singleToken, double: doubleToken}
	ctx = context.WithValue(ctx, dollarEscapeKey{}, tokens)
	return ctx, b.String()
}

// unescapeDollars restores $$ and $ from sentinel tokens.
func unescapeDollars(ctx context.Context, input string) string {
	if ctx == nil {
		return input
	}
	tokens, ok := ctx.Value(dollarEscapeKey{}).(dollarEscapeTokens)
	if !ok {
		return input
	}
	if tokens.double == "" && tokens.single == "" {
		return input
	}
	out := strings.ReplaceAll(input, tokens.double, "$$")
	out = strings.ReplaceAll(out, tokens.single, "$")
	return out
}

type quoteSpan struct {
	start int
	end   int
}

// singleQuoteSpans returns balanced single-quote spans in the input.
func singleQuoteSpans(input string) []quoteSpan {
	var spans []quoteSpan
	for i := 0; i < len(input); {
		if input[i] != '\'' {
			i++
			continue
		}
		next := strings.IndexByte(input[i+1:], '\'')
		if next < 0 {
			break
		}
		end := i + 1 + next
		spans = append(spans, quoteSpan{start: i, end: end})
		i = end + 1
	}
	return spans
}

func uniqueToken(input, base string) string {
	for {
		id := atomic.AddUint64(&dollarEscapeSeq, 1)
		token := fmt.Sprintf("%s%d__", base, id)
		if !strings.Contains(input, token) {
			return token
		}
	}
}
