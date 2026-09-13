package curtains

import "testing"

// ParseRole faqat chegirma berilishi mumkin bo'lgan rollarni taniydi. Rollar
// aniq kichik harfda (employee|manager) — "MANAGER" yoki "admin" kabi qiymat
// fail-closed xatoga olib borishi kerak (rules §5.6), ilgari noma'lum rol
// manager chegarasiga tenglab qo'yilgan edi (xavfsizlik teshigi).
func TestParseRole_OnlyKnownConstants(t *testing.T) {
	valid := map[string]UserRole{
		"employee": RoleEmployee,
		"manager":  RoleManager,
	}
	for in, want := range valid {
		got, ok := ParseRole(in)
		if !ok || got != want {
			t.Errorf("ParseRole(%q) = %q, %v; kutilgan %q, true", in, got, ok, want)
		}
	}

	for _, in := range []string{"", "admin", "MANAGER", "EMPLOYEE", " employee", "manager ", "director", "unknown"} {
		if got, ok := ParseRole(in); ok {
			t.Errorf("ParseRole(%q) = %q, true — qabul qilinmasligi kerak", in, got)
		}
	}
}

func TestUserRoleValid(t *testing.T) {
	valid := map[UserRole]bool{
		RoleEmployee: true,
		RoleManager:  true,
		RoleAdmin:    false, // "admin" — chegirma uchun ruxsat etilgan rol emas
		"":           false,
		"MANAGER":    false,
		"unknown":    false,
	}
	for r, want := range valid {
		if got := r.Valid(); got != want {
			t.Errorf("UserRole(%q).Valid() = %v; kutilgan %v", r, got, want)
		}
	}
}
