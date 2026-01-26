// Copyright 2025 syzkaller project authors. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

// TODO: fix a typo in the package name
package assessmenet

import (
	"slices"

	"github.com/google/syzkaller/pkg/aflow"
	"github.com/google/syzkaller/pkg/aflow/action/kernel"
	"github.com/google/syzkaller/pkg/aflow/ai"
	"github.com/google/syzkaller/pkg/aflow/tool/codeexpert"
	"github.com/google/syzkaller/pkg/aflow/tool/codesearcher"
)

type Inputs struct {
	CrashReport       string
	KernelRepo        string
	KernelCommit      string
	KernelConfig      string
	CodesearchToolBin string
}

type Outputs struct {
	BugExplanation string
	Explanation    string
	PriorityScore  int
}

func init() {
	commonTools := slices.Clip(append([]aflow.Tool{codeexpert.Tool}, codesearcher.Tools...))

	aflow.Register[Inputs, Outputs](
		ai.WorkflowPrioritization,
		"assess the severity and assign a priority score to a bug",
		&aflow.Flow{
			Root: aflow.Pipeline(
				kernel.Checkout,
				kernel.Build,
				codesearcher.PrepareIndex,
				&aflow.LLMAgent{
					Name:        "debugger",
					Model:       aflow.BestExpensiveModel,
					Reply:       "BugExplanation",
					Temperature: 1,
					Instruction: debuggingInstruction,
					Prompt:      debuggingPrompt,
					Tools:       commonTools,
				},
				&aflow.LLMAgent{
					Name:  "prioritizer",
					Model: aflow.GoodBalancedModel,
					Reply: "Explanation",
					Outputs: aflow.LLMOutputs[struct {
						PriorityScore int `jsonschema:"The priority score integer from -1 to 10."`
					}](),
					Temperature: 0,
					Instruction: prioritizationInstruction,
					Prompt:      prioritizationPrompt,
				},
			),
		},
	)
}

const debuggingInstruction = `
You are an experienced Linux kernel developer tasked with debugging a kernel crash root cause.
You need to provide a detailed explanation of the root cause.
Your final reply must contain only the explanation.

Call some codesearch tools first.
`

const debuggingPrompt = `
The crash is:

{{.CrashReport}}
`

// TODO(matejuk) write better prompt with a playbook
const prioritizationInstruction = `
You are an experienced Linux kernel maintainer.
Analyze the provided crash report and the root cause explanation.
Assign a priority score as an integer from -1 to 10 to this bug.

Score definitions:
* **-1**: Invalid report, false positive, or intentional behavior.
* **0**: Benign issue, noise, or documentation only.
* **1-3 (Low)**: Hard to trigger, cosmetic, or minor impact on non-core subsystems.
* **4-6 (Medium)**: Standard bugs, reliable crash in drivers, potential DOS.
* **7-9 (High)**: Core subsystem corruption, frequent crashes, security implications.
* **10 (Critical)**: Remote code execution, privilege escalation, or breaks build/boot completely.

Your final reply should explain the reasoning behind the score.
`

const prioritizationPrompt = `
Crash Report:
{{.CrashReport}}

Root Cause Analysis:
{{.BugExplanation}}
`
