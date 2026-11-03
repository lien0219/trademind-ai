package settings

// Default mail content is event-specific in all supported UI languages.
// Customized event/locale templates override these copies.
var localizedDefaultMailCopy = map[string]map[string][2]string{
	"en-US": {
		"auth.verify_code": {"Email verification", "Use this one-time code to verify your email address."},
		"auth.password_reset": {"Reset your password", "Use the verification code below to reset your password. Ignore this message if you did not request it."},
		"auth.registration_welcome": {"Welcome to TradeMind", "Your account is ready. Sign in to get started."},
		"tenant.invitation": {"Organization invitation", "You have been invited to join an organization. Open the invitation link to continue."},
		"tenant.member_added": {"Organization membership", "Your organization membership has been created."},
		"security.account_alert": {"Account security alert", "A security-related account change was recorded. Review your account if this was unexpected."},
		"task.failed": {"Task failure alert", "An automated task failed. Review the task details in the console."},
		"report.ready": {"Report ready", "Your requested report is available."},
		"subscription.expiry": {"Subscription expiry reminder", "Your subscription is approaching its expiration date."},
		"billing.payment": {"Payment confirmation", "Your payment has been recorded."},
	},
	"ja-JP": {
		"auth.verify_code": {"メール認証コード", "メールアドレスを確認するための認証コードです。"},
		"auth.password_reset": {"パスワードの再設定", "パスワード再設定用の確認コードです。心当たりがない場合は無視してください。"},
		"auth.registration_welcome": {"TradeMindへようこそ", "アカウントの作成が完了しました。ログインしてご利用ください。"},
		"tenant.invitation": {"組織への招待", "組織に招待されました。招待リンクから手続きを進めてください。"},
		"tenant.member_added": {"組織メンバー登録", "組織のメンバーとして登録されました。"},
		"security.account_alert": {"アカウントのセキュリティ通知", "アカウントのセキュリティに関する変更がありました。"},
		"task.failed": {"タスク失敗の通知", "自動処理タスクが失敗しました。管理画面で確認してください。"},
		"report.ready": {"レポートの準備完了", "ご依頼のレポートを確認できます。"},
		"subscription.expiry": {"サブスクリプションの期限通知", "サブスクリプションの有効期限が近づいています。"},
		"billing.payment": {"お支払い完了のお知らせ", "お支払いが記録されました。"},
	},
	"zh-TW": {
		"auth.verify_code": {"電子郵件驗證碼", "請使用下方一次性驗證碼完成電子郵件驗證。"},
		"auth.password_reset": {"重設密碼", "請使用下方驗證碼重設密碼。若非本人操作請忽略。"},
		"auth.registration_welcome": {"歡迎加入 TradeMind", "帳號已建立完成，請登入開始使用。"},
		"tenant.invitation": {"企業邀請", "您已受邀加入企業，請透過邀請連結完成操作。"},
		"tenant.member_added": {"企業成員通知", "您已成為企業成員。"},
		"security.account_alert": {"帳號安全提醒", "您的帳號發生安全相關變更，請確認是否為本人操作。"},
		"task.failed": {"任務失敗提醒", "自動化任務執行失敗，請前往管理後台查看。"},
		"report.ready": {"報表已完成", "您要求的報表已可供查看。"},
		"subscription.expiry": {"訂閱到期提醒", "您的訂閱即將到期。"},
		"billing.payment": {"付款成功通知", "您的付款已記錄。"},
	},
	"ko-KR": {
		"auth.verify_code": {"이메일 인증 코드", "이메일 주소 확인을 위해 아래 일회용 인증 코드를 사용하세요."},
		"auth.password_reset": {"비밀번호 재설정", "아래 인증 코드로 비밀번호를 재설정하세요. 요청하지 않았다면 무시하세요."},
		"auth.registration_welcome": {"TradeMind에 오신 것을 환영합니다", "계정 생성이 완료되었습니다. 로그인하여 시작하세요."},
		"tenant.invitation": {"조직 초대", "조직에 초대되었습니다. 초대 링크를 통해 가입을 완료하세요."},
		"tenant.member_added": {"조직 구성원 알림", "조직 구성원으로 등록되었습니다."},
		"security.account_alert": {"계정 보안 알림", "계정 보안 관련 변경이 발생했습니다. 본인의 활동인지 확인하세요."},
		"task.failed": {"작업 실패 알림", "자동화 작업이 실패했습니다. 관리 화면에서 확인하세요."},
		"report.ready": {"보고서 준비 완료", "요청하신 보고서를 확인할 수 있습니다."},
		"subscription.expiry": {"구독 만료 안내", "구독 만료일이 다가오고 있습니다."},
		"billing.payment": {"결제 완료 알림", "결제 내역이 기록되었습니다."},
	},
	"th-TH": {
		"auth.verify_code": {"รหัสยืนยันอีเมล", "ใช้รหัสแบบใช้ครั้งเดียวด้านล่างเพื่อยืนยันอีเมลของคุณ"},
		"auth.password_reset": {"รีเซ็ตรหัสผ่าน", "ใช้รหัสด้านล่างเพื่อรีเซ็ตรหัสผ่าน หากไม่ได้ร้องขอโปรดเพิกเฉย"},
		"auth.registration_welcome": {"ยินดีต้อนรับสู่ TradeMind", "บัญชีของคุณพร้อมใช้งานแล้ว กรุณาเข้าสู่ระบบ"},
		"tenant.invitation": {"คำเชิญเข้าร่วมองค์กร", "คุณได้รับคำเชิญให้เข้าร่วมองค์กร โปรดเปิดลิงก์คำเชิญเพื่อดำเนินการ"},
		"tenant.member_added": {"แจ้งการเป็นสมาชิกองค์กร", "คุณได้รับการเพิ่มเป็นสมาชิกองค์กรแล้ว"},
		"security.account_alert": {"แจ้งเตือนความปลอดภัยบัญชี", "มีการเปลี่ยนแปลงด้านความปลอดภัยของบัญชี โปรดตรวจสอบหากไม่ใช่คุณ"},
		"task.failed": {"แจ้งเตือนงานล้มเหลว", "งานอัตโนมัติดำเนินการไม่สำเร็จ โปรดตรวจสอบในระบบ"},
		"report.ready": {"รายงานพร้อมใช้งาน", "รายงานที่ร้องขอพร้อมให้ดูแล้ว"},
		"subscription.expiry": {"แจ้งเตือนการหมดอายุสมาชิก", "การสมัครสมาชิกของคุณใกล้หมดอายุ"},
		"billing.payment": {"ยืนยันการชำระเงิน", "ระบบบันทึกการชำระเงินของคุณแล้ว"},
	},
}

func localizedMailCopy(event, locale, fallbackTitle, fallbackBody string) (string, string) {
	translations, ok := localizedDefaultMailCopy[locale]
	if !ok {
		return fallbackTitle, fallbackBody
	}
	copy, ok := translations[event]
	if !ok {
		return fallbackTitle, fallbackBody
	}
	return copy[0], copy[1]
}
