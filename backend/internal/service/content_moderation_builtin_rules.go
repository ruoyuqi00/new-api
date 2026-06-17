package service

import "strings"

const (
	contentModerationBuiltInCategoryReverseEngineeringAbuse = "builtin/reverse_engineering_abuse"
	contentModerationBuiltInCategoryCredentialAbuse         = "builtin/credential_abuse"
	contentModerationBuiltInCategoryAutomationAbuse         = "builtin/automation_abuse"
)

type contentModerationBuiltInRuleHit struct {
	RuleID   string
	Category string
	Keyword  string
	Score    float64
}

type contentModerationBuiltInPhraseRule struct {
	id       string
	category string
	phrases  []string
}

var contentModerationBuiltInPhraseRules = []contentModerationBuiltInPhraseRule{
	{
		id:       "reverse_engineering_direct",
		category: contentModerationBuiltInCategoryReverseEngineeringAbuse,
		phrases: []string{
			"逆向破解",
			"反编译破解",
			"脱壳破解",
			"破解授权",
			"破解许可",
			"破解许可证",
			"绕过授权",
			"绕过许可",
			"绕过许可证",
			"绕过激活",
			"破解激活",
			"绕过付费",
			"破解付费",
			"绕过会员",
			"破解会员",
			"绕过订阅",
			"破解订阅",
			"制作注册机",
			"写注册机",
			"生成注册码",
			"注册码生成器",
			"license key generator",
			"serial key generator",
			"software cracking",
			"crack license",
			"bypass license",
			"bypass activation",
			"bypass paywall",
			"bypass subscription",
			"patch the binary to bypass",
		},
	},
	{
		id:       "credential_abuse_direct",
		category: contentModerationBuiltInCategoryCredentialAbuse,
		phrases: []string{
			"盗取账号",
			"盗号",
			"窃取 token",
			"窃取token",
			"盗取 token",
			"盗取token",
			"偷 cookie",
			"偷cookie",
			"导出别人 cookie",
			"导出别人cookie",
			"dump refresh token",
			"steal access token",
			"steal refresh token",
			"steal cookies",
			"session hijacking",
			"account takeover",
		},
	},
	{
		id:       "automation_abuse_direct",
		category: contentModerationBuiltInCategoryAutomationAbuse,
		phrases: []string{
			"绕过验证码",
			"绕过风控",
			"批量注册账号",
			"自动注册账号",
			"自动化养号",
			"撞库",
			"爆破密码",
			"credential stuffing",
			"captcha bypass",
			"bypass captcha",
			"bypass risk control",
			"mass account registration",
		},
	},
}

var contentModerationBuiltInReverseActions = []string{
	"破解",
	"绕过",
	"逆向",
	"反编译",
	"脱壳",
	"hook",
	"frida",
	"ida",
	"ghidra",
	"apktool",
	"decompile",
	"reverse engineer",
	"binary patch",
	"patch binary",
	"crack",
	"bypass",
	"keygen",
}

var contentModerationBuiltInReverseTargets = []string{
	"授权",
	"许可",
	"许可证",
	"注册机",
	"注册码",
	"激活",
	"会员",
	"付费",
	"订阅",
	"试用",
	"风控",
	"验证码",
	"签名校验",
	"完整性校验",
	"license",
	"licence",
	"activation",
	"subscription",
	"paywall",
	"trial",
	"drm",
	"serial",
	"captcha",
}

var contentModerationBuiltInCredentialActions = []string{
	"盗取",
	"窃取",
	"偷取",
	"抓取",
	"导出别人",
	"提取别人",
	"劫持",
	"dump",
	"steal",
	"exfiltrate",
	"hijack",
}

var contentModerationBuiltInCredentialTargets = []string{
	"账号",
	"账户",
	"cookie",
	"cookies",
	"token",
	"access token",
	"refresh token",
	"session",
	"session token",
	"oauth",
	"api key",
	"apikey",
}

var contentModerationBuiltInAutomationActions = []string{
	"绕过",
	"爆破",
	"撞库",
	"批量注册",
	"自动注册",
	"批量登录",
	"bypass",
	"bruteforce",
	"brute force",
	"credential stuffing",
	"mass register",
	"mass signup",
}

var contentModerationBuiltInAutomationTargets = []string{
	"验证码",
	"风控",
	"登录",
	"注册",
	"账号",
	"账户",
	"captcha",
	"risk control",
	"login",
	"signup",
	"registration",
	"account",
}

var contentModerationBuiltInInstructionTerms = []string{
	"帮我",
	"教我",
	"如何",
	"怎么",
	"步骤",
	"方案",
	"代码",
	"脚本",
	"实现",
	"生成",
	"编写",
	"制作",
	"写一个",
	"给我",
	"make",
	"write",
	"generate",
	"build",
	"implement",
	"steps",
	"how to",
	"show me",
	"give me",
}

var contentModerationBuiltInResearchAllowTerms = []string{
	"ctf",
	"capture the flag",
	"靶场",
	"授权测试",
	"合规测试",
	"防御",
	"防护",
	"检测",
	"审计",
	"漏洞报告",
	"malware analysis",
	"forensics",
	"defensive",
	"authorized test",
	"security audit",
}

func matchBuiltInRiskRule(text string) (contentModerationBuiltInRuleHit, bool) {
	normalized := normalizeBuiltInRiskText(text)
	if normalized == "" {
		return contentModerationBuiltInRuleHit{}, false
	}
	for _, rule := range contentModerationBuiltInPhraseRules {
		if phrase, ok := containsAnyTerm(normalized, rule.phrases); ok {
			return contentModerationBuiltInRuleHit{
				RuleID:   rule.id,
				Category: rule.category,
				Keyword:  phrase,
				Score:    1,
			}, true
		}
	}
	if isLikelyResearchContext(normalized) {
		return contentModerationBuiltInRuleHit{}, false
	}
	if hit, ok := matchBuiltInCompositeRule(
		normalized,
		"reverse_engineering_composite",
		contentModerationBuiltInCategoryReverseEngineeringAbuse,
		contentModerationBuiltInReverseActions,
		contentModerationBuiltInReverseTargets,
	); ok {
		return hit, true
	}
	if hit, ok := matchBuiltInCompositeRule(
		normalized,
		"credential_abuse_composite",
		contentModerationBuiltInCategoryCredentialAbuse,
		contentModerationBuiltInCredentialActions,
		contentModerationBuiltInCredentialTargets,
	); ok {
		return hit, true
	}
	return matchBuiltInCompositeRule(
		normalized,
		"automation_abuse_composite",
		contentModerationBuiltInCategoryAutomationAbuse,
		contentModerationBuiltInAutomationActions,
		contentModerationBuiltInAutomationTargets,
	)
}

func matchBuiltInCompositeRule(text string, ruleID string, category string, actions []string, targets []string) (contentModerationBuiltInRuleHit, bool) {
	action, hasAction := containsAnyTerm(text, actions)
	if !hasAction {
		return contentModerationBuiltInRuleHit{}, false
	}
	target, hasTarget := containsAnyTerm(text, targets)
	if !hasTarget {
		return contentModerationBuiltInRuleHit{}, false
	}
	if _, hasInstruction := containsAnyTerm(text, contentModerationBuiltInInstructionTerms); !hasInstruction {
		return contentModerationBuiltInRuleHit{}, false
	}
	return contentModerationBuiltInRuleHit{
		RuleID:   ruleID,
		Category: category,
		Keyword:  action + "+" + target,
		Score:    1,
	}, true
}

func isLikelyResearchContext(text string) bool {
	_, ok := containsAnyTerm(text, contentModerationBuiltInResearchAllowTerms)
	return ok
}

func normalizeBuiltInRiskText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"\r", " ",
		"\n", " ",
		"\t", " ",
		"　", " ",
		"：", ":",
		"，", ",",
		"。", ".",
		"（", "(",
		"）", ")",
		"【", "[",
		"】", "]",
		"“", "\"",
		"”", "\"",
		"‘", "'",
		"’", "'",
	)
	return strings.Join(strings.Fields(replacer.Replace(text)), " ")
}

func containsAnyTerm(text string, terms []string) (string, bool) {
	for _, term := range terms {
		term = normalizeBuiltInRiskText(term)
		if term == "" {
			continue
		}
		if strings.Contains(text, term) {
			return term, true
		}
	}
	return "", false
}
