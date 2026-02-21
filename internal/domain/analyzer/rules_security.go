package analyzer

// securityRules defines all security tier rules. Each rule has a unique ID and
// bit position within TierSecurity (bits 0-63).
var securityRules = []Rule{
	// === Text-only rules (AC pattern matching) ===
	{
		ID: "hardcoded_secret", Label: "Potential hardcoded secret or credential",
		Dimension: "secrets", Tier: TierSecurity, Bit: 0, Severity: SevCritical,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"password=", "password =", "password:", "passwd=", "passwd =",
			"secret=", "secret =", "secret:",
			"api_key=", "api_key =", "apikey=", "apikey =",
			"private_key=", "private_key =",
			"access_token=", "access_token =",
		},
	},
	{
		ID: "command_injection", Label: "Potential command injection via exec/system call",
		Dimension: "injection", Tier: TierSecurity, Bit: 1, Severity: SevCritical,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"exec.Command(", "os.system(", "subprocess.call(",
			"subprocess.Popen(", "subprocess.run(",
			"child_process.exec(", "child_process.spawn(",
		},
	},
	{
		ID: "weak_hash", Label: "MD5 or SHA1 used (weak for security purposes)",
		Dimension: "crypto", Tier: TierSecurity, Bit: 2, Severity: SevWarning,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"md5.New(", "md5.Sum(", "hashlib.md5(",
			"sha1.New(", "sha1.Sum(", "hashlib.sha1(",
			"crypto.createHash('md5')", "crypto.createHash('sha1')",
			"MD5.Create(", "SHA1.Create(",
		},
	},
	{
		ID: "insecure_random", Label: "math/rand used where crypto/rand may be needed",
		Dimension: "crypto", Tier: TierSecurity, Bit: 3, Severity: SevWarning,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			`"math/rand"`, "import random", "Math.random()",
		},
	},
	{
		ID: "sql_injection_keyword", Label: "Potential SQL injection via string concatenation",
		Dimension: "injection", Tier: TierSecurity, Bit: 4, Severity: SevCritical,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			`"SELECT " +`, `"INSERT " +`, `"UPDATE " +`, `"DELETE " +`,
			`"SELECT " %`, `"INSERT " %`, `"UPDATE " %`, `"DELETE " %`,
			`f"SELECT `, `f"INSERT `, `f"UPDATE `, `f"DELETE `,
			"\"SELECT \" +", "\"INSERT \" +", "\"UPDATE \" +", "\"DELETE \" +",
		},
	},
	{
		ID: "path_traversal_keyword", Label: "Potential path traversal via unvalidated input",
		Dimension: "injection", Tier: TierSecurity, Bit: 5, Severity: SevHigh,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{"../", "..\\"},
	},
	{
		ID: "hardcoded_ip", Label: "Hardcoded IP address",
		Dimension: "config", Tier: TierSecurity, Bit: 6, Severity: SevInfo,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"192.168.", "10.0.", "172.16.", "0.0.0.0",
		},
	},
	{
		ID: "insecure_http", Label: "Insecure HTTP URL (non-HTTPS)",
		Dimension: "transport", Tier: TierSecurity, Bit: 7, Severity: SevWarning,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{"http://"},
	},
	{
		ID: "debug_endpoint", Label: "Debug/admin endpoint exposed",
		Dimension: "exposure", Tier: TierSecurity, Bit: 8, Severity: SevHigh,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"/debug/pprof", "/admin", "/_debug", "/swagger",
		},
	},
	{
		ID: "sensitive_data_log", Label: "Sensitive data potentially logged",
		Dimension: "data", Tier: TierSecurity, Bit: 9, Severity: SevWarning,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"log.Print(password", "log.Printf(\"password",
			"log.Print(secret", "log.Printf(\"secret",
			"console.log(password", "console.log(secret",
			"print(password", "print(secret",
		},
	},
	{
		ID: "cors_wildcard", Label: "CORS wildcard allows any origin",
		Dimension: "exposure", Tier: TierSecurity, Bit: 10, Severity: SevWarning,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			`Access-Control-Allow-Origin", "*"`,
			`"Access-Control-Allow-Origin": "*"`,
			`cors.AllowAll(`,
		},
	},
	{
		ID: "eval_usage", Label: "Dynamic code execution via eval",
		Dimension: "injection", Tier: TierSecurity, Bit: 11, Severity: SevCritical,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{"eval(", "Function("},
	},
	{
		ID: "unsafe_deserialization", Label: "Unsafe deserialization",
		Dimension: "injection", Tier: TierSecurity, Bit: 12, Severity: SevCritical,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"pickle.loads(", "pickle.load(",
			"yaml.load(", "yaml.unsafe_load(",
			"Marshal.load(", "unserialize(",
		},
	},
	{
		ID: "insecure_tls", Label: "Insecure TLS configuration",
		Dimension: "transport", Tier: TierSecurity, Bit: 13, Severity: SevHigh,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"InsecureSkipVerify: true", "InsecureSkipVerify:true",
			"MinVersion: tls.VersionTLS10", "MinVersion: tls.VersionSSL",
			"ssl.PROTOCOL_TLSv1", "SSLv3",
		},
	},
	{
		ID: "disabled_tls_verify", Label: "TLS certificate verification disabled",
		Dimension: "transport", Tier: TierSecurity, Bit: 14, Severity: SevCritical,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"verify=False", "NODE_TLS_REJECT_UNAUTHORIZED",
			"CURLOPT_SSL_VERIFYPEER, 0", "CURLOPT_SSL_VERIFYPEER, false",
		},
	},
	{
		ID: "aws_credentials", Label: "AWS credentials in source code",
		Dimension: "secrets", Tier: TierSecurity, Bit: 15, Severity: SevCritical,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{"AKIA", "aws_secret_access_key", "aws_access_key_id"},
	},
	{
		ID: "jwt_secret_inline", Label: "JWT secret hardcoded in source",
		Dimension: "secrets", Tier: TierSecurity, Bit: 16, Severity: SevCritical,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"jwt.sign(", "jwt.encode(",
			"JWT_SECRET=", "JWT_SECRET =",
			"signingKey =", "signingKey=",
		},
	},
	{
		ID: "private_key_inline", Label: "Private key material in source",
		Dimension: "secrets", Tier: TierSecurity, Bit: 17, Severity: SevCritical,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"BEGIN RSA PRIVATE KEY", "BEGIN EC PRIVATE KEY",
			"BEGIN PRIVATE KEY", "BEGIN DSA PRIVATE KEY",
		},
	},
	{
		ID: "world_readable_perms", Label: "World-readable file permissions",
		Dimension: "config", Tier: TierSecurity, Bit: 18, Severity: SevWarning,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{"0777", "0666", "0755"},
	},
	{
		ID: "open_redirect_keyword", Label: "Potential open redirect",
		Dimension: "injection", Tier: TierSecurity, Bit: 19, Severity: SevHigh,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"redirect_url", "redirect_uri", "return_url", "next_url",
			"Redirect(r.URL.Query()", "redirect(request.args",
		},
	},

	// === Structural rules (AST walker) ===
	{
		ID: "defer_in_loop", Label: "defer inside loop body",
		Dimension: "resources", Tier: TierSecurity, Bit: 20, Severity: SevWarning,
		Kind: RuleStructural, CodeOnly: true,
		StructuralCheck: "checkDeferInLoop",
	},
	{
		ID: "ignored_error", Label: "Error return value assigned to blank identifier",
		Dimension: "errors", Tier: TierSecurity, Bit: 21, Severity: SevWarning,
		Kind: RuleStructural, CodeOnly: true,
		StructuralCheck: "checkIgnoredError",
	},
	{
		ID: "panic_in_lib", Label: "panic() called in library/non-main package",
		Dimension: "errors", Tier: TierSecurity, Bit: 22, Severity: SevWarning,
		Kind: RuleStructural, SkipMain: true, CodeOnly: true,
		StructuralCheck: "checkPanicInLib",
	},
	{
		ID: "unchecked_type_assertion", Label: "Type assertion without comma-ok pattern",
		Dimension: "errors", Tier: TierSecurity, Bit: 23, Severity: SevWarning,
		Kind: RuleStructural, CodeOnly: true,
		StructuralCheck: "checkUncheckedTypeAssert",
	},
	{
		ID: "sql_string_concat", Label: "SQL query built via string concatenation",
		Dimension: "injection", Tier: TierSecurity, Bit: 24, Severity: SevCritical,
		Kind: RuleStructural, SkipTest: true, CodeOnly: true,
		StructuralCheck: "checkSQLStringConcat",
	},
	{
		ID: "error_not_checked", Label: "Error return value not checked",
		Dimension: "errors", Tier: TierSecurity, Bit: 25, Severity: SevWarning,
		Kind: RuleStructural, CodeOnly: true,
		StructuralCheck: "checkErrorNotChecked",
	},
	{
		ID: "long_function", Label: "Function exceeds 100 lines",
		Dimension: "complexity", Tier: TierSecurity, Bit: 26, Severity: SevInfo,
		Kind: RuleStructural, CodeOnly: true,
		StructuralCheck: "checkLongFunction",
	},

	// === Composite rules (AC + AST confirmation) ===
	{
		ID: "exec_with_variable", Label: "exec.Command with non-literal argument",
		Dimension: "injection", Tier: TierSecurity, Bit: 30, Severity: SevCritical,
		Kind: RuleComposite, SkipTest: true, CodeOnly: true,
		StructuralCheck: "checkExecWithVariable",
		TextPatterns:    []string{"exec.Command(", "os.system(", "subprocess.call("},
	},
	{
		ID: "tainted_path_join", Label: "path.Join with potentially tainted argument",
		Dimension: "injection", Tier: TierSecurity, Bit: 31, Severity: SevHigh,
		Kind: RuleComposite, SkipTest: true, CodeOnly: true,
		StructuralCheck: "checkTaintedPathJoin",
		TextPatterns:    []string{"filepath.Join(", "path.Join(", "os.path.join("},
	},
	{
		ID: "format_string_injection", Label: "Format string with user-controlled input",
		Dimension: "injection", Tier: TierSecurity, Bit: 32, Severity: SevHigh,
		Kind: RuleComposite, SkipTest: true, CodeOnly: true,
		StructuralCheck: "checkFormatStringInjection",
		TextPatterns:    []string{"fmt.Sprintf(", "fmt.Fprintf(", "String.format("},
	},
	{
		ID: "template_unescaped", Label: "Unescaped template output",
		Dimension: "injection", Tier: TierSecurity, Bit: 33, Severity: SevHigh,
		Kind: RuleComposite, SkipTest: true, CodeOnly: true,
		StructuralCheck: "checkTemplateUnescaped",
		TextPatterns:    []string{"template.HTML(", "| safe", "| raw", "dangerouslySetInnerHTML"},
	},
	{
		ID: "regex_dos", Label: "Potentially catastrophic regex backtracking",
		Dimension: "denial", Tier: TierSecurity, Bit: 34, Severity: SevWarning,
		Kind: RuleComposite, SkipTest: true, CodeOnly: true,
		StructuralCheck: "checkRegexDos",
		TextPatterns:    []string{"regexp.Compile(", "regexp.MustCompile(", "new RegExp(", "re.compile("},
	},
}

// AllRules returns all defined rules across all tiers.
func AllRules() []Rule {
	// Currently only security tier; future tiers append here.
	all := make([]Rule, len(securityRules))
	copy(all, securityRules)
	return all
}

// TextRules returns only the rules that have text patterns (text + composite).
func TextRules() []Rule {
	var out []Rule
	for _, r := range securityRules {
		if len(r.TextPatterns) > 0 {
			out = append(out, r)
		}
	}
	return out
}

// StructuralRules returns only the rules that need AST walking (structural + composite).
func StructuralRules() []Rule {
	var out []Rule
	for _, r := range securityRules {
		if r.Kind == RuleStructural || r.Kind == RuleComposite {
			out = append(out, r)
		}
	}
	return out
}
