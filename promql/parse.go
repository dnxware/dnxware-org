// Copyright 2015 The dnxware Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package promql

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/dnxware/common/model"

	"github.com/dnxware/dnxware/pkg/labels"
	"github.com/dnxware/dnxware/pkg/value"
	"github.com/dnxware/dnxware/util/strutil"
)

type parser struct {
	lex       *lexer
	token     [3]item
	peekCount int
}

// ParseErr wraps a parsing error with line and position context.
// If the parsing input was a single line, line will be 0 and omitted
// from the error string.
type ParseErr struct {
	Line, Pos int
	Err       error
}

func (e *ParseErr) Error() string {
	if e.Line == 0 {
		return fmt.Sprintf("parse error at char %d: %s", e.Pos, e.Err)
	}
	return fmt.Sprintf("parse error at line %d, char %d: %s", e.Line, e.Pos, e.Err)
}

// ParseExpr returns the expression parsed from the input.
func ParseExpr(input string) (Expr, error) {
	p := newParser(input)

	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	err = p.typecheck(expr)
	return expr, err
}

// ParseMetric parses the input into a metric
func ParseMetric(input string) (m labels.Labels, err error) {
	p := newParser(input)
	defer p.recover(&err)

	m = p.metric()
	if p.peek().typ != ItemEOF {
		p.errorf("could not parse remaining input %.15q...", p.lex.input[p.lex.lastPos:])
	}
	return m, nil
}

// ParseMetricSelector parses the provided textual metric selector into a list of
// label matchers.
func ParseMetricSelector(input string) (m []*labels.Matcher, err error) {
	p := newParser(input)
	defer p.recover(&err)

	name := ""
	if t := p.peek().typ; t == ItemMetricIdentifier || t == ItemIdentifier {
		name = p.next().val
	}
	vs := p.VectorSelector(name)
	if p.peek().typ != ItemEOF {
		p.errorf("could not parse remaining input %.15q...", p.lex.input[p.lex.lastPos:])
	}
	return vs.LabelMatchers, nil
}

// newParser returns a new parser.
func newParser(input string) *parser {
	p := &parser{
		lex: lex(input),
	}
	return p
}

// parseExpr parses a single expression from the input.
func (p *parser) parseExpr() (expr Expr, err error) {
	defer p.recover(&err)

	for p.peek().typ != ItemEOF {
		if p.peek().typ == ItemComment {
			continue
		}
		if expr != nil {
			p.errorf("could not parse remaining input %.15q...", p.lex.input[p.lex.lastPos:])
		}
		expr = p.expr()
	}

	if expr == nil {
		p.errorf("no expression found in input")
	}
	return
}

// sequenceValue is an omittable value in a sequence of time series values.
type sequenceValue struct {
	value   float64
	omitted bool
}

func (v sequenceValue) String() string {
	if v.omitted {
		return "_"
	}
	return fmt.Sprintf("%f", v.value)
}

// parseSeriesDesc parses the description of a time series.
func parseSeriesDesc(input string) (labels.Labels, []sequenceValue, error) {
	p := newParser(input)
	p.lex.seriesDesc = true

	return p.parseSeriesDesc()
}

// parseSeriesDesc parses a description of a time series into its metric and value sequence.
func (p *parser) parseSeriesDesc() (m labels.Labels, vals []sequenceValue, err error) {
	defer p.recover(&err)

	m = p.metric()

	const ctx = "series values"
	for {
		for p.peek().typ == ItemSpace {
			p.next()
		}
		if p.peek().typ == ItemEOF {
			break
		}

		// Extract blanks.
		if p.peek().typ == ItemBlank {
			p.next()
			times := uint64(1)
			if p.peek().typ == ItemTimes {
				p.next()
				times, err = strconv.ParseUint(p.expect(ItemNumber, ctx).val, 10, 64)
				if err != nil {
					p.errorf("invalid repetition in %s: %s", ctx, err)
				}
			}
			for i := uint64(0); i < times; i++ {
				vals = append(vals, sequenceValue{omitted: true})
			}
			// This is to ensure that there is a space between this and the next number.
			// This is especially required if the next number is negative.
			if t := p.expectOneOf(ItemSpace, ItemEOF, ctx).typ; t == ItemEOF {
				break
			}
			continue
		}

		// Extract values.
		sign := 1.0
		if t := p.peek().typ; t == ItemSUB || t == ItemADD {
			if p.next().typ == ItemSUB {
				sign = -1
			}
		}
		var k float64
		if t := p.peek().typ; t == ItemNumber {
			k = sign * p.number(p.expect(ItemNumber, ctx).val)
		} else if t == ItemIdentifier && p.peek().val == "stale" {
			p.next()
			k = math.Float64frombits(value.StaleNaN)
		} else {
			p.errorf("expected number or 'stale' in %s but got %s (value: %s)", ctx, t.desc(), p.peek())
		}
		vals = append(vals, sequenceValue{
			value: k,
		})

		// If there are no offset repetitions specified, proceed with the next value.
		if t := p.peek(); t.typ == ItemSpace {
			// This ensures there is a space between every value.
			continue
		} else if t.typ == ItemEOF {
			break
		} else if t.typ != ItemADD && t.typ != ItemSUB {
			p.errorf("expected next value or relative expansion in %s but got %s (value: %s)", ctx, t.desc(), p.peek())
		}

		// Expand the repeated offsets into values.
		sign = 1.0
		if p.next().typ == ItemSUB {
			sign = -1.0
		}
		offset := sign * p.number(p.expect(ItemNumber, ctx).val)
		p.expect(ItemTimes, ctx)

		times, err := strconv.ParseUint(p.expect(ItemNumber, ctx).val, 10, 64)
		if err != nil {
			p.errorf("invalid repetition in %s: %s", ctx, err)
		}

		for i := uint64(0); i < times; i++ {
			k += offset
			vals = append(vals, sequenceValue{
				value: k,
			})
		}
		// This is to ensure that there is a space between this expanding notation
		// and the next number. This is especially required if the next number
		// is negative.
		if t := p.expectOneOf(ItemSpace, ItemEOF, ctx).typ; t == ItemEOF {
			break
		}
	}
	return m, vals, nil
}

// typecheck checks correct typing of the parsed statements or expression.
func (p *parser) typecheck(node Node) (err error) {
	defer p.recover(&err)

	p.checkType(node)
	return nil
}

// next returns the next token.
func (p *parser) next() item {
	if p.peekCount > 0 {
		p.peekCount--
	} else {
		t := p.lex.nextItem()
		// Skip comments.
		for t.typ == ItemComment {
			t = p.lex.nextItem()
		}
		p.token[0] = t
	}
	if p.token[p.peekCount].typ == ItemError {
		p.errorf("%s", p.token[p.peekCount].val)
	}
	return p.token[p.peekCount]
}

// peek returns but does not consume the next token.
func (p *parser) peek() item {
	if p.peekCount > 0 {
		return p.token[p.peekCount-1]
	}
	p.peekCount = 1

	t := p.lex.nextItem()
	// Skip comments.
	for t.typ == ItemComment {
		t = p.lex.nextItem()
	}
	p.token[0] = t
	return p.token[0]
}

// backup backs the input stream up one token.
func (p *parser) backup() {
	p.peekCount++
}

// errorf formats the error and terminates processing.
func (p *parser) errorf(format string, args ...interface{}) {
	p.error(errors.Errorf(format, args...))
}

// error terminates processing.
func (p *parser) error(err error) {
	perr := &ParseErr{
		Line: p.lex.lineNumber(),
		Pos:  p.lex.linePosition(),
		Err:  err,
	}
	if strings.Count(strings.TrimSpace(p.lex.input), "\n") == 0 {
		perr.Line = 0
	}
	panic(perr)
}

// expect consumes the next token and guarantees it has the required type.
func (p *parser) expect(exp ItemType, context string) item {
	token := p.next()
	if token.typ != exp {
		p.errorf("unexpected %s in %s, expected %s", token.desc(), context, exp.desc())
	}
	return token
}

// expectOneOf consumes the next token and guarantees it has one of the required types.
func (p *parser) expectOneOf(exp1, exp2 ItemType, context string) item {
	token := p.next()
	if token.typ != exp1 && token.typ != exp2 {
		p.errorf("unexpected %s in %s, expected %s or %s", token.desc(), context, exp1.desc(), exp2.desc())
	}
	return token
}

var errUnexpected = errors.New("unexpected error")

// recover is the handler that turns panics into returns from the top level of Parse.
func (p *parser) recover(errp *error) {
	e := recover()
	if _, ok := e.(runtime.Error); ok {
		// Print the stack trace but do not inhibit the running application.
		buf := make([]byte, 64<<10)
		buf = buf[:runtime.Stack(buf, false)]

		fmt.Fprintf(os.Stderr, "parser panic: %v\n%s", e, buf)
		*errp = errUnexpected
	} else if e != nil {
		*errp = e.(error)
	}
	p.lex.close()
}

// expr parses any expression.
func (p *parser) expr() Expr {
	// Parse the starting expression.
	expr := p.unaryExpr()

	// Loop through the operations and construct a binary operation tree based
	// on the operators' precedence.
	for {
		// If the next token is not an operator the expression is done.
		op := p.peek().typ
		if !op.isOperator() {
			// Check for subquery.
			if op == ItemLeftBracket {
				expr = p.subqueryOrRangeSelector(expr, false)
				if s, ok := expr.(*SubqueryExpr); ok {
					// Parse optional offset.
					if p.peek().typ == ItemOffset {
						offset := p.offset()
						s.Offset = offset
					}
				}
			}
			return expr
		}
		p.next() // Consume operator.

		// Parse optional operator matching options. Its validity
		// is checked in the type-checking stage.
		vecMatching := &VectorMatching{
			Card: CardOneToOne,
		}
		if op.isSetOperator() {
			vecMatching.Card = CardManyToMany
		}

		returnBool := false
		// Parse bool modifier.
		if p.peek().typ == ItemBool {
			if !op.isComparisonOperator() {
				p.errorf("bool modifier can only be used on comparison operators")
			}
			p.next()
			returnBool = true
		}

		// Parse ON/IGNORING clause.
		if p.peek().typ == ItemOn || p.peek().typ == ItemIgnoring {
			if p.peek().typ == ItemOn {
				vecMatching.On = true
			}
			p.next()
			vecMatching.MatchingLabels = p.labels()

			// Parse grouping.
			if t := p.peek().typ; t == ItemGroupLeft || t == ItemGroupRight {
				p.next()
				if t == ItemGroupLeft {
					vecMatching.Card = CardManyToOne
				} else {
					vecMatching.Card = CardOneToMany
				}
				if p.peek().typ == ItemLeftParen {
					vecMatching.Include = p.labels()
				}
			}
		}

		for _, ln := range vecMatching.MatchingLabels {
			for _, ln2 := range vecMatching.Include {
				if ln == ln2 && vecMatching.On {
					p.errorf("label %q must not occur in ON and GROUP clause at once", ln)
				}
			}
		}

		// Parse the next operand.
		rhs := p.unaryExpr()

		// Assign the new root based on the precedence of the LHS and RHS operators.
		expr = p.balance(expr, op, rhs, vecMatching, returnBool)
	}
}

func (p *parser) balance(lhs Expr, op ItemType, rhs Expr, vecMatching *VectorMatching, returnBool bool) *BinaryExpr {
	if lhsBE, ok := lhs.(*BinaryExpr); ok {
		precd := lhsBE.Op.precedence() - op.precedence()
		if (precd < 0) || (precd == 0 && op.isRightAssociative()) {
			balanced := p.balance(lhsBE.RHS, op, rhs, vecMatching, returnBool)
			if lhsBE.Op.isComparisonOperator() && !lhsBE.ReturnBool && balanced.Type() == ValueTypeScalar && lhsBE.LHS.Type() == ValueTypeScalar {
				p.errorf("comparisons between scalars must use BOOL modifier")
			}
			return &BinaryExpr{
				Op:             lhsBE.Op,
				LHS:            lhsBE.LHS,
				RHS:            balanced,
				VectorMatching: lhsBE.VectorMatching,
				ReturnBool:     lhsBE.ReturnBool,
			}
		}
	}
	if op.isComparisonOperator() && !returnBool && rhs.Type() == ValueTypeScalar && lhs.Type() == ValueTypeScalar {
		p.errorf("comparisons between scalars must use BOOL modifier")
	}
	return &BinaryExpr{
		Op:             op,
		LHS:            lhs,
		RHS:            rhs,
		VectorMatching: vecMatching,
		ReturnBool:     returnBool,
	}
}

// unaryExpr parses a unary expression.
//
//		<Vector_selector> | <Matrix_selector> | (+|-) <number_literal> | '(' <expr> ')'
//
func (p *parser) unaryExpr() Expr {
	switch t := p.peek(); t.typ {
	case ItemADD, ItemSUB:
		p.next()
		e := p.unaryExpr()

		// Simplify unary expressions for number literals.
		if nl, ok := e.(*NumberLiteral); ok {
			if t.typ == ItemSUB {
				nl.Val *= -1
			}
			return nl
		}
		return &UnaryExpr{Op: t.typ, Expr: e}

	case ItemLeftParen:
		p.next()
		e := p.expr()
		p.expect(ItemRightParen, "paren expression")

		return &ParenExpr{Expr: e}
	}
	e := p.primaryExpr()

	// Expression might be followed by a range selector.
	if p.peek().typ == ItemLeftBracket {
		e = p.subqueryOrRangeSelector(e, true)
	}

	// Parse optional offset.
	if p.peek().typ == ItemOffset {
		offset := p.offset()

		switch s := e.(type) {
		case *VectorSelector:
			s.Offset = offset
		case *MatrixSelector:
			s.Offset = offset
		case *SubqueryExpr:
			s.Offset = offset
		default:
			p.errorf("offset modifier must be preceded by an instant or range selector, but follows a %T instead", e)
		}
	}

	return e
}

// subqueryOrRangeSelector parses a Subquery based on given Expr (or)
// a Matrix (a.k.a. range) selector based on a given Vector selector.
//
//		<Vector_selector> '[' <duration> ']' | <Vector_selector> '[' <duration> ':' [<duration>] ']'
//
func (p *parser) subqueryOrRangeSelector(expr Expr, checkRange bool) Expr {
	ctx := "subquery selector"
	if checkRange {
		ctx = "range/subquery selector"
	}

	p.next()

	var erange time.Duration
	var err error

	erangeStr := p.expect(ItemDuration, ctx).val
	erange, err = parseDuration(erangeStr)
	if err != nil {
		p.error(err)
	}

	var itm item
	if checkRange {
		itm = p.expectOneOf(ItemRightBracket, ItemColon, ctx)
		if itm.typ == ItemRightBracket {
			// Range selector.
			vs, ok := expr.(*VectorSelector)
			if !ok {
				p.errorf("range specification must be preceded by a metric selector, but follows a %T instead", expr)
			}
			return &MatrixSelector{
				Name:          vs.Name,
				LabelMatchers: vs.LabelMatchers,
				Range:         erange,
			}
		}
	} else {
		itm = p.expect(ItemColon, ctx)
	}

	// Subquery.
	var estep time.Duration

	itm = p.expectOneOf(ItemRightBracket, ItemDuration, ctx)
	if itm.typ == ItemDuration {
		estepStr := itm.val
		estep, err = parseDuration(estepStr)
		if err != nil {
			p.error(err)
		}
		p.expect(ItemRightBracket, ctx)
	}

	return &SubqueryExpr{
		Expr:  expr,
		Range: erange,
		Step:  estep,
	}
}

// number parses a number.
func (p *parser) number(val string) float64 {
	n, err := strconv.ParseInt(val, 0, 64)
	f := float64(n)
	if err != nil {
		f, err = strconv.ParseFloat(val, 64)
	}
	if err != nil {
		p.errorf("error parsing number: %s", err)
	}
	return f
}

// primaryExpr parses a primary expression.
//
//		<metric_name> | <function_call> | <Vector_aggregation> | <literal>
//
func (p *parser) primaryExpr() Expr {
	switch t := p.next(); {
	case t.typ == ItemNumber:
		f := p.number(t.val)
		return &NumberLiteral{f}

	case t.typ == ItemString:
		return &StringLiteral{p.unquoteString(t.val)}

	case t.typ == ItemLeftBrace:
		// Metric selector without metric name.
		p.backup()
		return p.VectorSelector("")

	case t.typ == ItemIdentifier:
		// Check for function call.
		if p.peek().typ == ItemLeftParen {
			return p.call(t.val)
		}
		fallthrough // Else metric selector.

	case t.typ == ItemMetricIdentifier:
		return p.VectorSelector(t.val)

	case t.typ.isAggregator():
		p.backup()
		return p.aggrExpr()

	default:
		p.errorf("no valid expression found")
	}
	return nil
}

// labels parses a list of labelnames.
//
//		'(' <label_name>, ... ')'
//
func (p *parser) labels() []string {
	const ctx = "grouping opts"

	p.expect(ItemLeftParen, ctx)

	labels := []string{}
	if p.peek().typ != ItemRightParen {
		for {
			id := p.next()
			if !isLabel(id.val) {
				p.errorf("unexpected %s in %s, expected label", id.desc(), ctx)
			}
			labels = append(labels, id.val)

			if p.peek().typ != ItemComma {
				break
			}
			p.next()
		}
	}
	p.expect(ItemRightParen, ctx)

	return labels
}

// aggrExpr parses an aggregation expression.
//
//		<aggr_op> (<Vector_expr>) [by|without <labels>]
//		<aggr_op> [by|without <labels>] (<Vector_expr>)
//
func (p *parser) aggrExpr() *AggregateExpr {
	const ctx = "aggregation"

	agop := p.next()
	if !agop.typ.isAggregator() {
		p.errorf("expected aggregation operator but got %s", agop)
	}
	var grouping []string
	var without bool

	modifiersFirst := false

	if t := p.peek().typ; t == ItemBy || t == ItemWithout {
		if t == ItemWithout {
			without = true
		}
		p.next()
		grouping = p.labels()
		modifiersFirst = true
	}

	p.expect(ItemLeftParen, ctx)
	var param Expr
	if agop.typ.isAggregatorWithParam() {
		param = p.expr()
		p.expect(ItemComma, ctx)
	}
	e := p.expr()
	p.expect(ItemRightParen, ctx)

	if !modifiersFirst {
		if t := p.peek().typ; t == ItemBy || t == ItemWithout {
			if len(grouping) > 0 {
				p.errorf("aggregation must only contain one grouping clause")
			}
			if t == ItemWithout {
				without = true
			}
			p.next()
			grouping = p.labels()
		}
	}

	return &AggregateExpr{
		Op:       agop.typ,
		Expr:     e,
		Param:    param,
		Grouping: grouping,
		Without:  without,
	}
}

// call parses a function call.
//
//		<func_name> '(' [ <arg_expr>, ...] ')'
//
func (p *parser) call(name string) *Call {
	const ctx = "function call"

	fn, exist := getFunction(name)
	if !exist {
		p.errorf("unknown function with name %q", name)
	}

	p.expect(ItemLeftParen, ctx)
	// Might be call without args.
	if p.peek().typ == ItemRightParen {
		p.next() // Consume.
		return &Call{fn, nil}
	}

	var args []Expr
	for {
		e := p.expr()
		args = append(args, e)

		// Terminate if no more arguments.
		if p.peek().typ != ItemComma {
			break
		}
		p.next()
	}

	// Call must be closed.
	p.expect(ItemRightParen, ctx)

	return &Call{Func: fn, Args: args}
}

// labelSet parses a set of label matchers
//
//		'{' [ <labelname> '=' <match_string>, ... ] '}'
//
func (p *parser) labelSet() labels.Labels {
	set := []labels.Label{}
	for _, lm := range p.labelMatchers(ItemEQL) {
		set = append(set, labels.Label{Name: lm.Name, Value: lm.Value})
	}
	return labels.New(set...)
}

// labelMatchers parses a set of label matchers.
//
//		'{' [ <labelname> <match_op> <match_string>, ... ] '}'
//
func (p *parser) labelMatchers(operators ...ItemType) []*labels.Matcher {
	const ctx = "label matching"

	matchers := []*labels.Matcher{}

	p.expect(ItemLeftBrace, ctx)

	// Check if no matchers are provided.
	if p.peek().typ == ItemRightBrace {
		p.next()
		return matchers
	}

	for {
		label := p.expect(ItemIdentifier, ctx)

		op := p.next().typ
		if !op.isOperator() {
			p.errorf("expected label matching operator but got %s", op)
		}
		var validOp = false
		for _, allowedOp := range operators {
			if op == allowedOp {
				validOp = true
			}
		}
		if !validOp {
			p.errorf("operator must be one of %q, is %q", operators, op)
		}

		val := p.unquoteString(p.expect(ItemString, ctx).val)

		// Map the item to the respective match type.
		var matchType labels.MatchType
		switch op {
		case ItemEQL:
			matchType = labels.MatchEqual
		case ItemNEQ:
			matchType = labels.MatchNotEqual
		case ItemEQLRegex:
			matchType = labels.MatchRegexp
		case ItemNEQRegex:
			matchType = labels.MatchNotRegexp
		default:
			p.errorf("item %q is not a metric match type", op)
		}

		m, err := labels.NewMatcher(matchType, label.val, val)
		if err != nil {
			p.error(err)
		}

		matchers = append(matchers, m)

		if p.peek().typ == ItemIdentifier {
			p.errorf("missing comma before next identifier %q", p.peek().val)
		}

		// Terminate list if last matcher.
		if p.peek().typ != ItemComma {
			break
		}
		p.next()

		// Allow comma after each item in a multi-line listing.
		if p.peek().typ == ItemRightBrace {
			break
		}
	}

	p.expect(ItemRightBrace, ctx)

	return matchers
}

// metric parses a metric.
//
//		<label_set>
//		<metric_identifier> [<label_set>]
//
func (p *parser) metric() labels.Labels {
	name := ""
	var m labels.Labels

	t := p.peek().typ
	if t == ItemIdentifier || t == ItemMetricIdentifier {
		name = p.next().val
		t = p.peek().typ
	}
	if t != ItemLeftBrace && name == "" {
		p.errorf("missing metric name or metric selector")
	}
	if t == ItemLeftBrace {
		m = p.labelSet()
	}
	if name != "" {
		m = append(m, labels.Label{Name: labels.MetricName, Value: name})
		sort.Sort(m)
	}
	return m
}

// offset parses an offset modifier.
//
//		offset <duration>
//
func (p *parser) offset() time.Duration {
	const ctx = "offset"

	p.next()
	offi := p.expect(ItemDuration, ctx)

	offset, err := parseDuration(offi.val)
	if err != nil {
		p.error(err)
	}

	return offset
}

// VectorSelector parses a new (instant) vector selector.
//
//		<metric_identifier> [<label_matchers>]
//		[<metric_identifier>] <label_matchers>
//
func (p *parser) VectorSelector(name string) *VectorSelector {
	var matchers []*labels.Matcher
	// Parse label matching if any.
	if t := p.peek(); t.typ == ItemLeftBrace {
		matchers = p.labelMatchers(ItemEQL, ItemNEQ, ItemEQLRegex, ItemNEQRegex)
	}
	// Metric name must not be set in the label matchers and before at the same time.
	if name != "" {
		for _, m := range matchers {
			if m.Name == labels.MetricName {
				p.errorf("metric name must not be set twice: %q or %q", name, m.Value)
			}
		}
		// Set name label matching.
		m, err := labels.NewMatcher(labels.MatchEqual, labels.MetricName, name)
		if err != nil {
			panic(err) // Must not happen with metric.Equal.
		}
		matchers = append(matchers, m)
	}

	if len(matchers) == 0 {
		p.errorf("vector selector must contain label matchers or metric name")
	}
	// A Vector selector must contain at least one non-empty matcher to prevent
	// implicit selection of all metrics (e.g. by a typo).
	notEmpty := false
	for _, lm := range matchers {
		if !lm.Matches("") {
			notEmpty = true
			break
		}
	}
	if !notEmpty {
		p.errorf("vector selector must contain at least one non-empty matcher")
	}

	return &VectorSelector{
		Name:          name,
		LabelMatchers: matchers,
	}
}

// expectType checks the type of the node and raises an error if it
// is not of the expected type.
func (p *parser) expectType(node Node, want ValueType, context string) {
	t := p.checkType(node)
	if t != want {
		p.errorf("expected type %s in %s, got %s", documentedType(want), context, documentedType(t))
	}
}

// check the types of the children of each node and raise an error
// if they do not form a valid node.
//
// Some of these checks are redundant as the parsing stage does not allow
// them, but the costs are small and might reveal errors when making changes.
func (p *parser) checkType(node Node) (typ ValueType) {
	// For expressions the type is determined by their Type function.
	// Lists do not have a type but are not invalid either.
	switch n := node.(type) {
	case Expressions:
		typ = ValueTypeNone
	case Expr:
		typ = n.Type()
	default:
		p.errorf("unknown node type: %T", node)
	}

	// Recursively check correct typing for child nodes and raise
	// errors in case of bad typing.
	switch n := node.(type) {
	case *EvalStmt:
		ty := p.checkType(n.Expr)
		if ty == ValueTypeNone {
			p.errorf("evaluation statement must have a valid expression type but got %s", documentedType(ty))
		}

	case Expressions:
		for _, e := range n {
			ty := p.checkType(e)
			if ty == ValueTypeNone {
				p.errorf("expression must have a valid expression type but got %s", documentedType(ty))
			}
		}
	case *AggregateExpr:
		if !n.Op.isAggregator() {
			p.errorf("aggregation operator expected in aggregation expression but got %q", n.Op)
		}
		p.expectType(n.Expr, ValueTypeVector, "aggregation expression")
		if n.Op == ItemTopK || n.Op == ItemBottomK || n.Op == ItemQuantile {
			p.expectType(n.Param, ValueTypeScalar, "aggregation parameter")
		}
		if n.Op == ItemCountValues {
			p.expectType(n.Param, ValueTypeString, "aggregation parameter")
		}

	case *BinaryExpr:
		lt := p.checkType(n.LHS)
		rt := p.checkType(n.RHS)

		if !n.Op.isOperator() {
			p.errorf("binary expression does not support operator %q", n.Op)
		}
		if (lt != ValueTypeScalar && lt != ValueTypeVector) || (rt != ValueTypeScalar && rt != ValueTypeVector) {
			p.errorf("binary expression must contain only scalar and instant vector types")
		}

		if (lt != ValueTypeVector || rt != ValueTypeVector) && n.VectorMatching != nil {
			if len(n.VectorMatching.MatchingLabels) > 0 {
				p.errorf("vector matching only allowed between instant vectors")
			}
			n.VectorMatching = nil
		} else {
			// Both operands are Vectors.
			if n.Op.isSetOperator() {
				if n.VectorMatching.Card == CardOneToMany || n.VectorMatching.Card == CardManyToOne {
					p.errorf("no grouping allowed for %q operation", n.Op)
				}
				if n.VectorMatching.Card != CardManyToMany {
					p.errorf("set operations must always be many-to-many")
				}
			}
		}

		if (lt == ValueTypeScalar || rt == ValueTypeScalar) && n.Op.isSetOperator() {
			p.errorf("set operator %q not allowed in binary scalar expression", n.Op)
		}

	case *Call:
		nargs := len(n.Func.ArgTypes)
		if n.Func.Variadic == 0 {
			if nargs != len(n.Args) {
				p.errorf("expected %d argument(s) in call to %q, got %d", nargs, n.Func.Name, len(n.Args))
			}
		} else {
			na := nargs - 1
			if na > len(n.Args) {
				p.errorf("expected at least %d argument(s) in call to %q, got %d", na, n.Func.Name, len(n.Args))
			} else if nargsmax := na + n.Func.Variadic; n.Func.Variadic > 0 && nargsmax < len(n.Args) {
				p.errorf("expected at most %d argument(s) in call to %q, got %d", nargsmax, n.Func.Name, len(n.Args))
			}
		}

		for i, arg := range n.Args {
			if i >= len(n.Func.ArgTypes) {
				i = len(n.Func.ArgTypes) - 1
			}
			p.expectType(arg, n.Func.ArgTypes[i], fmt.Sprintf("call to function %q", n.Func.Name))
		}

	case *ParenExpr:
		p.checkType(n.Expr)

	case *UnaryExpr:
		if n.Op != ItemADD && n.Op != ItemSUB {
			p.errorf("only + and - operators allowed for unary expressions")
		}
		if t := p.checkType(n.Expr); t != ValueTypeScalar && t != ValueTypeVector {
			p.errorf("unary expression only allowed on expressions of type scalar or instant vector, got %q", documentedType(t))
		}

	case *SubqueryExpr:
		ty := p.checkType(n.Expr)
		if ty != ValueTypeVector {
			p.errorf("subquery is only allowed on instant vector, got %s in %q instead", ty, n.String())
		}

	case *NumberLiteral, *MatrixSelector, *StringLiteral, *VectorSelector:
		// Nothing to do for terminals.

	default:
		p.errorf("unknown node type: %T", node)
	}
	return
}

func (p *parser) unquoteString(s string) string {
	unquoted, err := strutil.Unquote(s)
	if err != nil {
		p.errorf("error unquoting string %q: %s", s, err)
	}
	return unquoted
}

func parseDuration(ds string) (time.Duration, error) {
	dur, err := model.ParseDuration(ds)
	if err != nil {
		return 0, err
	}
	if dur == 0 {
		return 0, errors.New("duration must be greater than 0")
	}
	return time.Duration(dur), nil
}
