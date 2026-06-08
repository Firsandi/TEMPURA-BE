package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type BrevoSender struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type BrevoRecipient struct {
	Email string `json:"email"`
}

type BrevoEmailPayload struct {
	Sender      BrevoSender      `json:"sender"`
	To          []BrevoRecipient `json:"to"`
	Subject     string           `json:"subject"`
	HtmlContent string           `json:"htmlContent"`
}

func sendEmailViaBrevo(toEmail, subject, htmlBody string) error {
	apiKey := os.Getenv("BREVO_API_KEY")
	senderEmail := os.Getenv("BREVO_SENDER_EMAIL")
	senderName := os.Getenv("BREVO_SENDER_NAME")

	if senderName == "" {
		senderName = "Tempura Admin"
	}

	payload := BrevoEmailPayload{
		Sender: BrevoSender{
			Name:  senderName,
			Email: senderEmail,
		},
		To: []BrevoRecipient{
			{
				Email: toEmail,
			},
		},
		Subject:     subject,
		HtmlContent: htmlBody,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal email payload: %v", err)
	}

	req, err := http.NewRequest("POST", "https://api.brevo.com/v3/smtp/email", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return fmt.Errorf("failed to create http request: %v", err)
	}

	req.Header.Set("accept", "application/json")
	req.Header.Set("api-key", apiKey)
	req.Header.Set("content-type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send http request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("brevo api returned non-success status code: %d", resp.StatusCode)
	}

	return nil
}

func SendAccountEmail(toEmail, password string) error {
	apiKey := os.Getenv("BREVO_API_KEY")
	senderEmail := os.Getenv("BREVO_SENDER_EMAIL")

	if apiKey == "" || senderEmail == "" {
		// Log warning but don't fail, useful for local testing without Brevo configured
		fmt.Println("Warning: Brevo configuration is missing. Cannot send email.")
		fmt.Printf("Generated Account: Email=%s, Password=%s\n", toEmail, password)
		return nil
	}

	subject := "Informasi Akun Pegawai Tempura"
	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="margin: 0; padding: 0; background-color: #f4f4f5; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;">
    <table width="100%%" cellpadding="0" cellspacing="0" role="presentation" style="margin: 0; padding: 40px 0; background-color: #f4f4f5;">
        <tr>
            <td align="center">
                <table width="100%%" cellpadding="0" cellspacing="0" role="presentation" style="max-width: 500px; margin: 0 auto; background-color: #121212; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);">
                    <tr>
                        <td style="padding: 40px 30px; text-align: center;">
                            <h1 style="margin: 0 0 30px; font-size: 24px; font-weight: bold; color: #FFB800; letter-spacing: 2px;">TEMPURA</h1>
                            
                            <h2 style="margin: 0 0 16px; font-size: 20px; font-weight: 600; color: #FFFFFF;">Akun Pegawai Baru</h2>
                            
                            <p style="margin: 0 0 30px; font-size: 14px; line-height: 24px; color: #A1A1AA;">
                                Selamat bergabung! Akun pegawai Anda di sistem Tempura telah berhasil dibuat. Berikut adalah kredensial login Anda:
                            </p>
                            
                            <table width="100%%" cellpadding="0" cellspacing="0" role="presentation">
                                <tr>
                                    <td align="center">
                                        <div style="background-color: #1A1A1A; border-radius: 8px; padding: 24px; margin-bottom: 30px; border: 1px solid #27272A; text-align: left;">
                                            <p style="margin: 0 0 8px; font-size: 11px; font-weight: bold; color: #71717A; letter-spacing: 1px; text-transform: uppercase;">EMAIL</p>
                                            <p style="margin: 0 0 20px; font-size: 16px; font-weight: 500; color: #FFFFFF;">%s</p>
                                            
                                            <p style="margin: 0 0 8px; font-size: 11px; font-weight: bold; color: #71717A; letter-spacing: 1px; text-transform: uppercase;">KATA SANDI SEMENTARA</p>
                                            <p style="margin: 0; font-size: 18px; font-weight: bold; color: #FFB800; letter-spacing: 2px;">%s</p>
                                        </div>
                                    </td>
                                </tr>
                            </table>
                            
                            <p style="margin: 0; font-size: 12px; line-height: 20px; color: #71717A;">
                                Harap segera masuk ke aplikasi dan <strong>ubah kata sandi Anda</strong> demi keamanan akun.
                            </p>
                            
                            <hr style="border: 0; border-top: 1px solid #27272A; margin: 30px 0;">
                            
                            <p style="margin: 0; font-size: 11px; color: #52525B;">
                                &copy; 2026 Tim Proyek IT UNEJ - Aplikasi Tempura
                            </p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>`, toEmail, password)

	return sendEmailViaBrevo(toEmail, subject, htmlBody)
}

func SendOTPEmail(toEmail, otpCode string) error {
	apiKey := os.Getenv("BREVO_API_KEY")
	senderEmail := os.Getenv("BREVO_SENDER_EMAIL")

	if apiKey == "" || senderEmail == "" {
		fmt.Printf("SIMULASI EMAIL OTP: Kode OTP untuk %s: %s\n", toEmail, otpCode)
		return nil
	}

	subject := "Kode Verifikasi Reset Password - Tempura"
	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="margin: 0; padding: 0; background-color: #f4f4f5; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;">
    <table width="100%%" cellpadding="0" cellspacing="0" role="presentation" style="margin: 0; padding: 40px 0; background-color: #f4f4f5;">
        <tr>
            <td align="center">
                <table width="100%%" cellpadding="0" cellspacing="0" role="presentation" style="max-width: 500px; margin: 0 auto; background-color: #121212; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);">
                    <tr>
                        <td style="padding: 40px 30px; text-align: center;">
                            <h1 style="margin: 0 0 30px; font-size: 24px; font-weight: bold; color: #FFB800; letter-spacing: 2px;">TEMPURA</h1>
                            
                            <h2 style="margin: 0 0 16px; font-size: 20px; font-weight: 600; color: #FFFFFF;">Lupa Kata Sandi?</h2>
                            
                            <p style="margin: 0 0 30px; font-size: 14px; line-height: 24px; color: #A1A1AA;">
                                Kami menerima permintaan untuk mereset kata sandi akun kamu. Masukkan kode berikut pada aplikasi untuk melanjutkan:
                            </p>
                            
                            <table width="100%%" cellpadding="0" cellspacing="0" role="presentation">
                                <tr>
                                    <td align="center">
                                        <div style="background-color: #1A1A1A; border-radius: 8px; padding: 24px; margin-bottom: 30px; border: 1px solid #27272A;">
                                            <p style="margin: 0 0 12px; font-size: 11px; font-weight: bold; color: #71717A; letter-spacing: 1px; text-transform: uppercase;">KODE VERIFIKASI</p>
                                            <p style="margin: 0; font-size: 36px; font-weight: bold; color: #FFB800; letter-spacing: 8px;">%s</p>
                                        </div>
                                    </td>
                                </tr>
                            </table>
                            
                            <p style="margin: 0; font-size: 12px; line-height: 20px; color: #71717A;">
                                Kode ini berlaku selama <strong>1 jam</strong>. Jika kamu tidak merasa meminta perubahan ini, mohon abaikan email ini.
                            </p>
                            
                            <hr style="border: 0; border-top: 1px solid #27272A; margin: 30px 0;">
                            
                            <p style="margin: 0; font-size: 11px; color: #52525B;">
                                &copy; 2026 Tim Proyek IT UNEJ - Aplikasi Tempura
                            </p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>`, otpCode)

	return sendEmailViaBrevo(toEmail, subject, htmlBody)
}
