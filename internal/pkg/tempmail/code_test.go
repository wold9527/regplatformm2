package tempmail

import "testing"

func TestExtractVerificationCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		subject string
		body    string
		want    string
	}{
		{
			name:    "openai_subject_tail_digits",
			subject: "Your ChatGPT code is 798357",
			want:    "798357",
		},
		{
			name: "gemini_body_keyword_alnum",
			body: "Your Google verification code is L655U3",
			want: "L655U3",
		},
		{
			name: "gemini_html_block_code",
			body: `<html><body><p>Your one-time verification code is:</p><div>DZJW4P</div></body></html>`,
			want: "DZJW4P",
		},
		{
			name: "quoted_printable_body",
			body: "Your verification code=20is=20AB12C3",
			want: "AB12C3",
		},
		{
			name: "numeric_body_without_keyword_context",
			body: "Use this one-time passcode on the next page: 456789",
			want: "456789",
		},
		{
			name: "seven_digit_body_code",
			body: "Your backup verification number is 1234567",
			want: "1234567",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ExtractVerificationCode(tc.subject, tc.body); got != tc.want {
				t.Fatalf("ExtractVerificationCode() = %q, want %q", got, tc.want)
			}
		})
	}
}
