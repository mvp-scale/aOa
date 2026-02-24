package analyzer

// securityRules defines hardcoded fallback rules using declarative Structural blocks.
// Bit assignments match the YAML source of truth (recon/rules/security.yaml).
// Prefer LoadRulesFromFS() for YAML-driven rules.
var securityRules = []Rule{
	// === Injection dimension (bits 0-7, 20-24) ===
	{
		ID: "command_injection", Label: "Potential command injection via exec/system call",
		Dimension: "injection", Tier: TierSecurity, Bit: 0, Severity: SevCritical,
		Kind: RuleComposite, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"exec.Command(", "os.system(", "subprocess.call(",
			"subprocess.Popen(", "subprocess.run(",
			"child_process.exec(", "child_process.spawn(",
		},
		Structural: &StructuralBlock{
			Match:            "call",
			ReceiverContains: []string{"exec.Command", "os.system", "subprocess.call", "subprocess.Popen", "child_process.exec"},
		},
	},
	{
		ID: "sql_injection_keyword", Label: "Potential SQL injection via string concatenation",
		Dimension: "injection", Tier: TierSecurity, Bit: 1, Severity: SevCritical,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			`"SELECT " +`, `"INSERT " +`, `"UPDATE " +`, `"DELETE " +`,
			`"SELECT " %`, `"INSERT " %`, `"UPDATE " %`, `"DELETE " %`,
			`f"SELECT `, `f"INSERT `, `f"UPDATE `, `f"DELETE `,
		},
	},
	{
		ID: "eval_usage", Label: "Dynamic code execution via eval",
		Dimension: "injection", Tier: TierSecurity, Bit: 2, Severity: SevCritical,
		Kind: RuleComposite, SkipTest: true, CodeOnly: true,
		Regex:        `\beval\s*\(`,
		TextPatterns: []string{"eval(", "Function("},
		Structural: &StructuralBlock{
			Match:            "call",
			ReceiverContains: []string{"eval", "Function"},
		},
	},
	{
		ID: "unsafe_deserialization", Label: "Unsafe deserialization",
		Dimension: "injection", Tier: TierSecurity, Bit: 3, Severity: SevCritical,
		Kind: RuleComposite, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"pickle.loads(", "pickle.load(",
			"yaml.load(", "yaml.unsafe_load(",
			"Marshal.load(", "unserialize(",
		},
		Structural: &StructuralBlock{
			Match:            "call",
			ReceiverContains: []string{"pickle.loads", "pickle.load", "yaml.load", "yaml.unsafe_load", "Marshal.load", "unserialize"},
		},
	},
	{
		ID: "path_traversal_keyword", Label: "Potential path traversal via unvalidated input",
		Dimension: "injection", Tier: TierSecurity, Bit: 4, Severity: SevHigh,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		Regex:        `\.\.[/\\]\w`,
		TextPatterns: []string{"../", "..\\"},
	},
	{
		ID: "open_redirect_keyword", Label: "Potential open redirect",
		Dimension: "injection", Tier: TierSecurity, Bit: 5, Severity: SevHigh,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"redirect_url", "redirect_uri", "return_url", "next_url",
			"Redirect(r.URL.Query()", "redirect(request.args",
		},
	},
	{
		ID: "ldap_injection", Label: "LDAP filter built with string concatenation",
		Dimension: "injection", Tier: TierSecurity, Bit: 6, Severity: SevHigh,
		Kind: RuleComposite, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"ldap.Search(", "ldap.NewSearchRequest(",
			"ldap_search(", "(cn=",
		},
		Structural: &StructuralBlock{
			Match:            "call",
			ReceiverContains: []string{"ldap.Search", "ldap.NewSearchRequest", "ldap_search"},
		},
	},
	{
		ID: "xxe_injection", Label: "XML parser without disabling external entities",
		Dimension: "injection", Tier: TierSecurity, Bit: 7, Severity: SevHigh,
		Kind: RuleComposite, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"xml.NewDecoder(", "xml.Unmarshal(",
			"etree.NewDocument(", "lxml.etree", "XMLReader(",
		},
		Structural: &StructuralBlock{
			Match:            "call",
			ReceiverContains: []string{"xml.NewDecoder", "xml.Unmarshal", "etree.NewDocument", "XMLReader"},
		},
	},

	// === Secrets dimension (bits 8-14) ===
	{
		ID: "hardcoded_secret", Label: "Potential hardcoded secret or credential",
		Dimension: "secrets", Tier: TierSecurity, Bit: 8, Severity: SevCritical,
		Kind: RuleComposite, SkipTest: true, CodeOnly: true,
		Regex: `(?:password|passwd|secret|api_?key|private_key|access_token)\s*[:=]\s*['"]?\S{4,}`,
		TextPatterns: []string{
			"password=", "password =", "password:", "passwd=", "passwd =",
			"secret=", "secret =", "secret:",
			"api_key=", "api_key =", "apikey=", "apikey =",
			"private_key=", "private_key =",
			"access_token=", "access_token =",
		},
		Structural: &StructuralBlock{
			Match:        "assignment",
			NameContains: []string{"password", "secret", "api_key", "token", "passwd"},
			ValueType:    "string_literal",
		},
	},
	{
		ID: "aws_credentials", Label: "AWS credentials in source code",
		Dimension: "secrets", Tier: TierSecurity, Bit: 9, Severity: SevCritical,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		Regex:        `AKIA[0-9A-Z]{16}`,
		TextPatterns: []string{"AKIA", "aws_secret_access_key", "aws_access_key_id"},
	},
	{
		ID: "jwt_secret_inline", Label: "JWT secret hardcoded in source",
		Dimension: "secrets", Tier: TierSecurity, Bit: 10, Severity: SevCritical,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"jwt.sign(", "jwt.encode(",
			"JWT_SECRET=", "JWT_SECRET =",
			"signingKey =", "signingKey=",
		},
	},
	{
		ID: "private_key_inline", Label: "Private key material in source",
		Dimension: "secrets", Tier: TierSecurity, Bit: 11, Severity: SevCritical,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		Regex: `-----BEGIN\s+(RSA\s+|EC\s+|DSA\s+|OPENSSH\s+)?PRIVATE\s+KEY-----`,
		TextPatterns: []string{
			"BEGIN RSA PRIVATE KEY", "BEGIN EC PRIVATE KEY",
			"BEGIN PRIVATE KEY", "BEGIN DSA PRIVATE KEY",
		},
	},
	{
		ID: "connection_string_creds", Label: "Connection string with embedded credentials",
		Dimension: "secrets", Tier: TierSecurity, Bit: 12, Severity: SevCritical,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		Regex: `://\w+:[^@/]{4,}@`,
		TextPatterns: []string{
			"://root:", "://admin:", "://postgres:", "://sa:",
			"mongodb+srv://",
		},
	},
	{
		ID: "oauth_client_secret", Label: "OAuth client secret inline",
		Dimension: "secrets", Tier: TierSecurity, Bit: 13, Severity: SevCritical,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"client_secret=", "client_secret =",
			"CLIENT_SECRET=", "CLIENT_SECRET =",
		},
	},
	{
		ID: "webhook_secret_inline", Label: "Webhook secret inline",
		Dimension: "secrets", Tier: TierSecurity, Bit: 14, Severity: SevCritical,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"webhook_secret=", "webhook_secret =",
			"signing_secret=", "signing_secret =",
		},
	},

	// === Crypto dimension (bits 15-18) ===
	{
		ID: "weak_hash", Label: "MD5 or SHA1 used (weak for security purposes)",
		Dimension: "crypto", Tier: TierSecurity, Bit: 15, Severity: SevWarning,
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
		Dimension: "crypto", Tier: TierSecurity, Bit: 16, Severity: SevWarning,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			`"math/rand"`, "import random", "Math.random()",
		},
	},
	{
		ID: "ecb_mode", Label: "ECB block cipher mode (insecure)",
		Dimension: "crypto", Tier: TierSecurity, Bit: 17, Severity: SevHigh,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"cipher.NewECB", "AES/ECB", "MODE_ECB",
			"createCipheriv('aes-128-ecb'",
		},
	},
	{
		ID: "deprecated_tls", Label: "Deprecated TLS version configured",
		Dimension: "crypto", Tier: TierSecurity, Bit: 18, Severity: SevHigh,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"tls.VersionTLS10", "tls.VersionSSL",
			"ssl.PROTOCOL_TLSv1", "SSLv3", "TLSv1_METHOD",
		},
	},

	// === Structural rules from other tiers (quality, performance) ===
	{
		ID: "defer_in_loop", Label: "defer inside loop body",
		Dimension: "resources", Tier: TierPerformance, Bit: 0, Severity: SevWarning,
		Kind: RuleStructural, CodeOnly: true,
		Structural: &StructuralBlock{Match: "defer", Inside: "for_loop"},
	},
	{
		ID: "ignored_error", Label: "Error return value assigned to blank identifier",
		Dimension: "errors", Tier: TierQuality, Bit: 0, Severity: SevWarning,
		Kind: RuleStructural, CodeOnly: true,
		Structural: &StructuralBlock{
			Match:        "assignment",
			NameContains: []string{"_"},
			HasArg:       &ArgSpec{Type: []string{"call"}},
		},
		SkipLangs: []string{"python", "javascript", "typescript", "tsx", "ruby"},
	},
	{
		ID: "panic_in_lib", Label: "panic() called in library/non-main package",
		Dimension: "errors", Tier: TierQuality, Bit: 1, Severity: SevWarning,
		Kind: RuleStructural, SkipMain: true, CodeOnly: true,
		Structural: &StructuralBlock{Match: "call", TextContains: []string{"panic("}},
	},
	{
		ID: "unchecked_type_assertion", Label: "Type assertion without comma-ok pattern",
		Dimension: "errors", Tier: TierQuality, Bit: 2, Severity: SevWarning,
		Kind: RuleStructural, CodeOnly: true,
		Structural: &StructuralBlock{Match: "type_assertion", WithoutSibling: "comma_ok"},
		SkipLangs: []string{"python", "javascript", "typescript", "tsx", "ruby", "java", "c", "cpp", "rust"},
	},
	{
		ID: "sql_string_concat", Label: "SQL query built via string concatenation",
		Dimension: "injection", Tier: TierSecurity, Bit: 20, Severity: SevCritical,
		Kind: RuleStructural, SkipTest: true, CodeOnly: true,
		Structural: &StructuralBlock{
			Match:            "call",
			ReceiverContains: []string{"query", "execute", "exec", "prepare", "raw", "cursor"},
			HasArg: &ArgSpec{
				Type:         []string{"string_concat", "format_call", "template_string"},
				TextContains: []string{"SELECT", "INSERT", "UPDATE", "DELETE", "DROP"},
			},
		},
	},
	{
		ID: "error_not_checked", Label: "Error return value not checked",
		Dimension: "errors", Tier: TierQuality, Bit: 3, Severity: SevWarning,
		Kind: RuleStructural, CodeOnly: true,
		Structural: &StructuralBlock{Match: "call", WithoutSibling: "error_check"},
		SkipLangs: []string{"python", "javascript", "typescript", "tsx", "ruby", "java", "c", "cpp", "rust"},
	},
	{
		ID: "long_function", Label: "Function exceeds 100 lines",
		Dimension: "complexity", Tier: TierQuality, Bit: 4, Severity: SevInfo,
		Kind: RuleStructural, CodeOnly: true,
		Structural: &StructuralBlock{Match: "function", LineThreshold: 100},
	},

	// === Composite rules (AC + structural confirmation) ===
	{
		ID: "exec_with_variable", Label: "exec.Command with non-literal argument",
		Dimension: "injection", Tier: TierSecurity, Bit: 21, Severity: SevCritical,
		Kind: RuleComposite, SkipTest: true, CodeOnly: true,
		Structural: &StructuralBlock{
			Match:            "call",
			ReceiverContains: []string{"exec.Command", "os.system", "subprocess.call"},
			HasArg:           &ArgSpec{Type: []string{"identifier", "call"}},
		},
		TextPatterns: []string{"exec.Command(", "os.system(", "subprocess.call("},
	},
	{
		ID: "tainted_path_join", Label: "path.Join with potentially tainted argument",
		Dimension: "injection", Tier: TierSecurity, Bit: 22, Severity: SevHigh,
		Kind: RuleComposite, SkipTest: true, CodeOnly: true,
		Structural: &StructuralBlock{
			Match:            "call",
			ReceiverContains: []string{"filepath.Join", "path.Join", "os.path.join"},
			HasArg:           &ArgSpec{Type: []string{"identifier", "call"}},
		},
		TextPatterns: []string{"filepath.Join(", "path.Join(", "os.path.join("},
	},
	{
		ID: "format_string_injection", Label: "Format string with user-controlled input",
		Dimension: "injection", Tier: TierSecurity, Bit: 23, Severity: SevHigh,
		Kind: RuleComposite, SkipTest: true, CodeOnly: true,
		Structural: &StructuralBlock{
			Match:            "call",
			ReceiverContains: []string{"fmt.Sprintf", "fmt.Fprintf", "String.format"},
			HasArg:           &ArgSpec{Type: []string{"identifier", "call"}},
		},
		TextPatterns: []string{"fmt.Sprintf(", "fmt.Fprintf(", "String.format("},
	},
	{
		ID: "template_unescaped", Label: "Unescaped template output",
		Dimension: "injection", Tier: TierSecurity, Bit: 24, Severity: SevHigh,
		Kind: RuleComposite, SkipTest: true, CodeOnly: true,
		Structural: &StructuralBlock{
			Match:            "call",
			ReceiverContains: []string{"template.HTML", "template.JS", "template.URL"},
		},
		TextPatterns: []string{"template.HTML(", "| safe", "| raw", "dangerouslySetInnerHTML"},
	},
	{
		ID: "regex_dos", Label: "Potentially catastrophic regex backtracking",
		Dimension: "denial", Tier: TierSecurity, Bit: 25, Severity: SevWarning,
		Kind: RuleComposite, SkipTest: true, CodeOnly: true,
		Structural: &StructuralBlock{
			Match:            "call",
			ReceiverContains: []string{"regexp.Compile", "regexp.MustCompile", "RegExp", "re.compile"},
		},
		TextPatterns: []string{"regexp.Compile(", "regexp.MustCompile(", "new RegExp(", "re.compile("},
	},

	// === Transport dimension (bits 26-28) ===
	{
		ID: "insecure_tls", Label: "Insecure TLS configuration",
		Dimension: "transport", Tier: TierSecurity, Bit: 26, Severity: SevHigh,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		Regex: `InsecureSkipVerify\s*:\s*true`,
		TextPatterns: []string{
			"InsecureSkipVerify: true", "InsecureSkipVerify:true",
			"MinVersion: tls.VersionTLS10", "MinVersion: tls.VersionSSL",
		},
	},
	{
		ID: "disabled_tls_verify", Label: "TLS certificate verification disabled",
		Dimension: "transport", Tier: TierSecurity, Bit: 27, Severity: SevCritical,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"verify=False", "NODE_TLS_REJECT_UNAUTHORIZED",
			"CURLOPT_SSL_VERIFYPEER, 0", "CURLOPT_SSL_VERIFYPEER, false",
		},
	},
	{
		ID: "insecure_http", Label: "Insecure HTTP URL (non-HTTPS)",
		Dimension: "transport", Tier: TierSecurity, Bit: 28, Severity: SevWarning,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		Regex:        `http://[a-zA-Z0-9][\w.-]+`,
		TextPatterns: []string{"http://"},
	},

	// === Exposure dimension (bits 29-30) ===
	{
		ID: "debug_endpoint", Label: "Debug/admin endpoint exposed",
		Dimension: "exposure", Tier: TierSecurity, Bit: 29, Severity: SevHigh,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"/debug/pprof", "/admin", "/_debug", "/swagger",
		},
	},
	{
		ID: "cors_wildcard", Label: "CORS wildcard allows any origin",
		Dimension: "exposure", Tier: TierSecurity, Bit: 30, Severity: SevWarning,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			`Access-Control-Allow-Origin", "*"`,
			`"Access-Control-Allow-Origin": "*"`,
			`cors.AllowAll(`,
		},
	},

	// === Config dimension (bits 31-32) ===
	{
		ID: "hardcoded_ip", Label: "Hardcoded IP address",
		Dimension: "config", Tier: TierSecurity, Bit: 31, Severity: SevInfo,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		Regex: `\b(?:192\.168|10\.0|172\.(?:1[6-9]|2[0-9]|3[01])|0\.0\.0\.0)\.\d{1,3}`,
		TextPatterns: []string{
			"192.168.", "10.0.", "172.16.", "0.0.0.0",
		},
	},
	{
		ID: "world_readable_perms", Label: "World-readable file permissions",
		Dimension: "config", Tier: TierSecurity, Bit: 32, Severity: SevWarning,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		Regex:        `0[67][0-7]{2}`,
		TextPatterns: []string{"0777", "0666", "0755"},
	},

	// === Data dimension (bit 33) ===
	{
		ID: "sensitive_data_log", Label: "Sensitive data potentially logged",
		Dimension: "data", Tier: TierSecurity, Bit: 33, Severity: SevWarning,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"log.Print(password", "log.Printf(\"password",
			"log.Print(secret", "log.Printf(\"secret",
			"console.log(password", "console.log(secret",
			"print(password", "print(secret",
		},
	},

	// === Auth dimension (bits 34-36) ===
	{
		ID: "missing_csrf", Label: "POST/PUT handler without CSRF protection",
		Dimension: "auth", Tier: TierSecurity, Bit: 34, Severity: SevWarning,
		Kind: RuleComposite, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			".POST(", ".Put(", "HandleFunc(\"/api", `methods=["POST"]`,
		},
		Structural: &StructuralBlock{
			Match:            "call",
			ReceiverContains: []string{"POST", "Put", "HandleFunc"},
		},
	},
	{
		ID: "insecure_password_compare", Label: "Insecure password comparison (not constant-time)",
		Dimension: "auth", Tier: TierSecurity, Bit: 35, Severity: SevHigh,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"== password", "== hash", "password ==", "hmac.Equal",
		},
	},
	{
		ID: "missing_security_headers", Label: "Missing security headers",
		Dimension: "auth", Tier: TierSecurity, Bit: 36, Severity: SevWarning,
		Kind: RuleText, SkipTest: true, CodeOnly: true,
		TextPatterns: []string{
			"X-Frame-Options", "Content-Security-Policy",
			"Strict-Transport-Security", "X-Content-Type-Options",
		},
	},
}

// AllRules returns all hardcoded rules as a fallback.
// Prefer LoadRulesFromFS() for YAML-driven rules.
func AllRules() []Rule {
	all := make([]Rule, len(securityRules))
	copy(all, securityRules)
	return all
}

// TextRules returns only the rules that have text patterns (text + composite).
func TextRules() []Rule {
	return FilterTextRules(securityRules)
}

// StructuralRules returns only the rules that need AST walking (structural + composite).
func StructuralRules() []Rule {
	return FilterStructuralRules(securityRules)
}

// FilterTextRules returns rules with text patterns from a given slice.
func FilterTextRules(rules []Rule) []Rule {
	var out []Rule
	for _, r := range rules {
		if len(r.TextPatterns) > 0 {
			out = append(out, r)
		}
	}
	return out
}

// FilterStructuralRules returns rules needing AST walking from a given slice.
func FilterStructuralRules(rules []Rule) []Rule {
	var out []Rule
	for _, r := range rules {
		if r.Kind == RuleStructural || r.Kind == RuleComposite {
			out = append(out, r)
		}
	}
	return out
}
