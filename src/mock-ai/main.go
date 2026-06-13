// Command mock-ai is a tiny deterministic stand-in for an AI report generator.
//
// It exposes POST /generate which accepts a JSON body describing a financial dataset
// and returns a JSON body containing a generated markdown report. The output is fully
// deterministic (no API key, no network) so the workshop runs anywhere.
//
// Optionally, if AI_PROVIDER=openai and OPENAI_API_KEY is set, it forwards the prompt to
// the OpenAI Chat Completions API instead. This is the "real provider toggle" — it is not
// required for the workshop and is provided as an extension.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type generateRequest struct {
	Title        string `json:"title"`
	Dataset      string `json:"dataset"`
	Instructions string `json:"instructions"`
}

type generateResponse struct {
	Report   string `json:"report"`
	Provider string `json:"provider"`
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/generate", handleGenerate)

	addr := ":8080"
	log.Printf("mock-ai listening on %s (provider=%s)", addr, providerName())
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}

func providerName() string {
	if strings.EqualFold(os.Getenv("AI_PROVIDER"), "openai") && os.Getenv("OPENAI_API_KEY") != "" {
		return "openai"
	}
	return "mock"
}

func handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}
	var req generateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		req.Title = "Untitled Report"
	}

	var report string
	provider := providerName()
	if provider == "openai" {
		report, err = generateWithOpenAI(req)
		if err != nil {
			log.Printf("openai call failed, falling back to mock: %v", err)
			report = generateMock(req)
			provider = "mock(fallback)"
		}
	} else {
		report = generateMock(req)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(generateResponse{Report: report, Provider: provider})
}

// ── Financial parsing helpers ────────────────────────────────────────────────────────────

type financialLine struct {
	Label string
	Value float64
}

// parseDataset extracts "Label: Value" pairs from the free-form dataset string.
func parseDataset(dataset string) []financialLine {
	var result []financialLine
	for _, line := range strings.Split(dataset, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.LastIndex(line, ":")
		if idx < 0 {
			continue
		}
		label := strings.TrimSpace(line[:idx])
		value, ok := parseFinancialValue(strings.TrimSpace(line[idx+1:]))
		if !ok {
			continue
		}
		result = append(result, financialLine{Label: label, Value: value})
	}
	return result
}

// parseFinancialValue parses strings like "$1.20M", "550K", "400000", "-5B".
func parseFinancialValue(s string) (float64, bool) {
	s = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(s, "$", ""), ",", ""))
	if s == "" {
		return 0, false
	}
	multiplier := 1.0
	switch strings.ToUpper(s[len(s)-1:]) {
	case "B":
		multiplier, s = 1e9, s[:len(s)-1]
	case "M":
		multiplier, s = 1e6, s[:len(s)-1]
	case "K":
		multiplier, s = 1e3, s[:len(s)-1]
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, false
	}
	return v * multiplier, true
}

// formatValue formats a number as a compact dollar amount.
func formatValue(v float64) string {
	abs := math.Abs(v)
	neg := ""
	if v < 0 {
		neg = "-"
	}
	switch {
	case abs >= 1e9:
		return fmt.Sprintf("%s$%.2fB", neg, abs/1e9)
	case abs >= 1e6:
		return fmt.Sprintf("%s$%.2fM", neg, abs/1e6)
	case abs >= 1e3:
		return fmt.Sprintf("%s$%.1fK", neg, abs/1e3)
	default:
		return fmt.Sprintf("%s$%.0f", neg, abs)
	}
}

// findValue returns the first line whose label contains any of the given keywords.
func findValue(lines []financialLine, keywords ...string) float64 {
	for _, l := range lines {
		ll := strings.ToLower(l.Label)
		for _, kw := range keywords {
			if strings.Contains(ll, strings.ToLower(kw)) {
				return l.Value
			}
		}
	}
	return 0
}

func profitabilityWord(margin float64) string {
	switch {
	case margin > 20:
		return "a strong"
	case margin > 10:
		return "a healthy"
	case margin > 0:
		return "a marginally profitable"
	default:
		return "a loss-making"
	}
}

// ── generateMock ─────────────────────────────────────────────────────────────────────────

// generateMock produces a structured report. It parses the dataset for key:value financial
// lines, computes derived metrics (net margin, expense ratio), and emits @METRIC/@CHART/@TABLE
// directive lines that the worker renders as visual elements in the PDF, followed by
// well-reasoned prose sections.
func generateMock(req generateRequest) string {
	lines := parseDataset(req.Dataset)

	// ── Identify key figures ───────────────────────────────────────────────
	revenue := findValue(lines, "revenue", "sales", "turnover")
	cogs := findValue(lines, "cost of good", "cogs", "cost of sale")
	opex := findValue(lines, "operating", "opex", "overhead")
	netProfit := findValue(lines, "net profit", "net income")
	if netProfit == 0 {
		netProfit = findValue(lines, "profit")
	}
	totalCosts := cogs + opex

	// ── Compute summary metrics for the cards ─────────────────────────────
	type metricSpec struct {
		label    string
		value    float64
		fmtted   string
		positive bool
	}
	var metrics []metricSpec
	if revenue > 0 {
		metrics = append(metrics, metricSpec{"Revenue", revenue, formatValue(revenue), true})
	}
	if netProfit != 0 {
		metrics = append(metrics, metricSpec{"Net Profit", netProfit, formatValue(netProfit), netProfit > 0})
	}
	if revenue > 0 && netProfit != 0 {
		margin := netProfit / revenue * 100
		metrics = append(metrics, metricSpec{"Net Margin", margin, fmt.Sprintf("%.1f%%", margin), margin > 10})
	}
	if revenue > 0 && totalCosts > 0 {
		expRatio := totalCosts / revenue * 100
		metrics = append(metrics, metricSpec{"Expense Ratio", expRatio, fmt.Sprintf("%.1f%%", expRatio), expRatio < 75})
	}
	// Fallback: use first four raw lines as cards
	if len(metrics) == 0 {
		for i, l := range lines {
			if i >= 4 {
				break
			}
			metrics = append(metrics, metricSpec{l.Label, l.Value, formatValue(l.Value), l.Value > 0})
		}
	}

	var b strings.Builder

	// ── Directive lines (consumed by worker, not rendered as prose) ────────
	for _, m := range metrics {
		pos := "neutral"
		if m.positive {
			pos = "positive"
		}
		fmt.Fprintf(&b, "@METRIC:%s|%.4f|%s|%s\n", m.label, m.value, m.fmtted, pos)
	}
	for _, l := range lines {
		fmt.Fprintf(&b, "@CHART:%s|%.4f\n", l.Label, l.Value)
	}
	for _, l := range lines {
		fmt.Fprintf(&b, "@TABLE:%s|%s\n", l.Label, formatValue(l.Value))
	}

	// ── Executive Summary ─────────────────────────────────────────────────
	fmt.Fprintf(&b, "\n## Executive Summary\n\n")
	if revenue > 0 {
		if netProfit != 0 {
			margin := netProfit / revenue * 100
			totalCostsPct := 0.0
			if totalCosts > 0 {
				totalCostsPct = totalCosts / revenue * 100
			}
			if req.Instructions != "" {
				fmt.Fprintf(&b, "Per the analysis focus — %s — ", strings.ToLower(req.Instructions))
			}
			fmt.Fprintf(&b,
				"the dataset shows %s in revenue generating a net profit of %s (%.1f%% margin). "+
					"Total costs represent %.1f%% of revenue, placing the business in %s position.\n\n",
				formatValue(revenue), formatValue(netProfit), margin,
				totalCostsPct, profitabilityWord(margin),
			)
		} else {
			fmt.Fprintf(&b, "The dataset reports %s in revenue across %d line item(s).\n\n",
				formatValue(revenue), len(lines))
		}
	} else if len(lines) > 0 {
		total := 0.0
		for _, l := range lines {
			total += l.Value
		}
		if req.Instructions != "" {
			fmt.Fprintf(&b, "Analysis focus: %s. ", req.Instructions)
		}
		fmt.Fprintf(&b, "The %d reported figures sum to %s.\n\n", len(lines), formatValue(total))
	} else {
		fmt.Fprintf(&b, "No structured financial data was provided.\n\n")
	}

	// ── Key Findings ──────────────────────────────────────────────────────
	fmt.Fprintf(&b, "## Key Findings\n\n")
	if revenue > 0 {
		fmt.Fprintf(&b, "- Revenue of %s forms the top-line basis for all ratios in this report.\n", formatValue(revenue))
	}
	if cogs > 0 && revenue > 0 {
		fmt.Fprintf(&b, "- Cost of goods represents %.1f%% of revenue (%.2fx coverage ratio).\n",
			cogs/revenue*100, revenue/cogs)
	}
	if opex > 0 && revenue > 0 {
		fmt.Fprintf(&b, "- Operating expenses of %s account for %.1f%% of revenue.\n",
			formatValue(opex), opex/revenue*100)
	}
	if netProfit != 0 && revenue > 0 {
		margin := netProfit / revenue * 100
		switch {
		case margin > 20:
			fmt.Fprintf(&b, "- Net margin of %.1f%% indicates a high-margin operation with strong unit economics.\n", margin)
		case margin > 8:
			fmt.Fprintf(&b, "- Net margin of %.1f%% is within a healthy range for most sectors.\n", margin)
		case margin > 0:
			fmt.Fprintf(&b, "- Net margin of %.1f%% is positive but thin; further cost optimisation may improve outcomes.\n", margin)
		default:
			fmt.Fprintf(&b, "- Negative net margin of %.1f%% signals that costs exceed revenue — immediate attention recommended.\n", margin)
		}
	}
	if revenue == 0 && len(lines) > 0 {
		fmt.Fprintf(&b, "- No standard revenue line was identified; interpret figures relative to the dataset context.\n")
	}

	// ── Recommendations ───────────────────────────────────────────────────
	fmt.Fprintf(&b, "\n## Recommendations\n\n")
	if totalCosts > 0 && revenue > 0 && totalCosts/revenue > 0.7 {
		fmt.Fprintf(&b, "1. Prioritise cost reduction: the current expense ratio (%.0f%%) leaves limited headroom for growth investment.\n",
			totalCosts/revenue*100)
	} else {
		fmt.Fprintf(&b, "1. Maintain the current cost discipline and re-evaluate the cost structure quarterly.\n")
	}
	if netProfit > 0 {
		fmt.Fprintf(&b, "2. Reinvest a portion of the %s net profit into revenue-generating initiatives.\n", formatValue(netProfit))
	} else {
		fmt.Fprintf(&b, "2. Focus on revenue-growth levers to improve the top line before targeting margin expansion.\n")
	}
	fmt.Fprintf(&b, "3. Re-run this report next quarter to track trends over time.\n")

	fmt.Fprintf(&b, "\n---\n\n_This is a workshop demo. Figures are illustrative only._\n")
	return b.String()
}

// generateWithOpenAI forwards the prompt to OpenAI. Optional extension path.
func generateWithOpenAI(req generateRequest) (string, error) {
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}
	prompt := fmt.Sprintf(
		"Write a concise financial report titled %q. Instructions: %s. Dataset:\n%s",
		req.Title, req.Instructions, req.Dataset)

	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a financial analyst that writes clear markdown reports."},
			{"role": "user", "content": prompt},
		},
	}
	buf, _ := json.Marshal(payload)

	httpReq, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+os.Getenv("OPENAI_API_KEY"))

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai returned %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("openai returned no choices")
	}
	return parsed.Choices[0].Message.Content, nil
}
