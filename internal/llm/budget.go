package llm

// BudgetTracker tracks token usage against a fixed limit.
type BudgetTracker struct {
	limit int
	used  int
}

// NewBudgetTracker creates a BudgetTracker with the given token limit.
func NewBudgetTracker(limit int) *BudgetTracker {
	return &BudgetTracker{limit: limit}
}

// Used returns the number of tokens consumed so far.
func (b *BudgetTracker) Used() int { return b.used }

// Remaining returns how many tokens are left in the budget.
func (b *BudgetTracker) Remaining() int {
	r := b.limit - b.used
	if r < 0 {
		return 0
	}
	return r
}

// CanAfford returns true if estimated tokens fit within the remaining budget.
func (b *BudgetTracker) CanAfford(estimated int) bool {
	return b.used+estimated <= b.limit
}

// Record adds the given token count to cumulative usage.
func (b *BudgetTracker) Record(tokens int) { b.used += tokens }

// EstimateTokens approximates the token count for text using the
// standard GPT approximation of 1 token ≈ 4 characters.
func EstimateTokens(text string) int {
	n := len(text) / 4
	if n == 0 && len(text) > 0 {
		return 1
	}
	return n
}
