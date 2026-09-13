// Package pii markazlashgan PII maskalash qoidalarini saqlaydi.
package pii

import (
	"regexp"
	"strings"
)

var (
	phoneCountryRE = regexp.MustCompile(`(?:\+998|998)[ -]?[0-9]{2}[ -]?[0-9]{3}[ -]?[0-9]{2}[ -]?[0-9]{2}`)
	phoneLocalRE   = regexp.MustCompile(`[0-9]{2}[ -]?[0-9]{3}[ -]?[0-9]{2}[ -]?[0-9]{2}`)
)

// MaskPhone telefonning qidiruv va qo'ng'iroq uchun yetarli qismini qoldiradi.
func MaskPhone(phone string) string {
	digits := make([]byte, 0, len(phone))
	for i := 0; i < len(phone); i++ {
		if phone[i] >= '0' && phone[i] <= '9' {
			digits = append(digits, phone[i])
		}
	}
	if len(digits) >= 12 && strings.HasPrefix(string(digits), "998") {
		return "+998****" + string(digits[len(digits)-4:])
	}
	if len(digits) >= 9 {
		return "****" + string(digits[len(digits)-4:])
	}
	return "****"
}

// Mask eski adapterlar uchun MaskPhone nomining qisqa ko'rinishidir.
func Mask(phone string) string { return MaskPhone(phone) }

// MaskName ismni model va loglarda to'liq saqlamaslik uchun bosh harflarga qisqartiradi.
func MaskName(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return ""
	}
	initials := make([]string, 0, len(parts))
	for _, part := range parts {
		runes := []rune(part)
		if len(runes) > 0 {
			initials = append(initials, string(runes[0])+".")
		}
	}
	return strings.Join(initials, " ")
}

// Scrub matndagi O'zbekiston telefon raqamlarini maskalaydi.
func Scrub(text string) string {
	text = phoneCountryRE.ReplaceAllStringFunc(text, MaskPhone)
	return phoneLocalRE.ReplaceAllStringFunc(text, MaskPhone)
}

// ScrubToolResult tool natijasini maskalash uchun Scrub bilan bir xil siyosatni ishlatadi.
func ScrubToolResult(text string) string { return Scrub(text) }
