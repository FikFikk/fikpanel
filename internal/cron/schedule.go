package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// cronMacros memetakan alias standar crontab ke ekspresi 5-kolom
var cronMacros = map[string]string{
	"@yearly":   "0 0 1 1 *",
	"@annually": "0 0 1 1 *",
	"@monthly":  "0 0 1 * *",
	"@weekly":   "0 0 * * 0",
	"@daily":    "0 0 * * *",
	"@midnight": "0 0 * * *",
	"@hourly":   "0 * * * *",
}

var monthAliases = map[string]int{
	"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4,
	"MAY": 5, "JUN": 6, "JUL": 7, "AUG": 8,
	"SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12,
}

var dowAliases = map[string]int{
	"SUN": 0, "MON": 1, "TUE": 2, "WED": 3,
	"THU": 4, "FRI": 5, "SAT": 6,
}

// parsedSchedule menyimpan hasil parsing valid ekspresi crontab 5-kolom
type parsedSchedule struct {
	minutes       uint64 // bit 0..59
	hours         uint32 // bit 0..23
	days          uint32 // bit 1..31
	lastDay       bool   // mendukung token 'L' (hari terakhir bulan)
	months        uint16 // bit 1..12
	dows          uint8  // bit 0..6 (0=Minggu)
	domRestricted bool
	dowRestricted bool
}

// NormalizeSpec mengubah alias crontab (seperti @daily) menjadi ekspresi 5-kolom bersih
func NormalizeSpec(spec string) string {
	trimmed := strings.TrimSpace(spec)
	if expanded, ok := cronMacros[strings.ToLower(trimmed)]; ok {
		return expanded
	}
	return strings.Join(strings.Fields(trimmed), " ")
}

// ValidateSpec memastikan ekspresi crontab 5-kolom valid secara sintaksis dan rentang nilai
func ValidateSpec(spec string) error {
	_, err := parseSchedule(spec)
	return err
}

// parseSchedule membedah dan memvalidasi ekspresi 5-kolom crontab standar (mendukung *, */S, A-B, A-B/S, A/S, list koma, L, nama bulan/hari, dan @macro)
func parseSchedule(spec string) (*parsedSchedule, error) {
	norm := NormalizeSpec(spec)
	if norm == "" {
		return nil, fmt.Errorf("ekspresi jadwal crontab tidak boleh kosong")
	}
	fields := strings.Fields(norm)
	if len(fields) != 5 {
		return nil, fmt.Errorf("ekspresi crontab harus tepat 5 kolom (menit jam tanggal bulan hari), didapat %d kolom", len(fields))
	}

	minMask, _, err := parseFieldMask(fields[0], 0, 59, nil, false, false)
	if err != nil {
		return nil, fmt.Errorf("kolom menit tidak valid (%q): %w", fields[0], err)
	}
	hrMask, _, err := parseFieldMask(fields[1], 0, 23, nil, false, false)
	if err != nil {
		return nil, fmt.Errorf("kolom jam tidak valid (%q): %w", fields[1], err)
	}
	domMask, hasLastDay, err := parseFieldMask(fields[2], 1, 31, nil, false, true)
	if err != nil {
		return nil, fmt.Errorf("kolom tanggal tidak valid (%q): %w", fields[2], err)
	}
	monMask, _, err := parseFieldMask(fields[3], 1, 12, monthAliases, false, false)
	if err != nil {
		return nil, fmt.Errorf("kolom bulan tidak valid (%q): %w", fields[3], err)
	}
	dowMask, _, err := parseFieldMask(fields[4], 0, 7, dowAliases, true, false)
	if err != nil {
		return nil, fmt.Errorf("kolom hari tidak valid (%q): %w", fields[4], err)
	}

	return &parsedSchedule{
		minutes:       minMask,
		hours:         uint32(hrMask),
		days:          uint32(domMask),
		lastDay:       hasLastDay,
		months:        uint16(monMask),
		dows:          uint8(dowMask),
		domRestricted: fields[2] != "*",
		dowRestricted: fields[4] != "*",
	}, nil
}

// parseFieldMask mem-parsing satu kolom cron menjadi bitmask nilai valid
func parseFieldMask(field string, minVal, maxVal int, aliases map[string]int, isDOW, allowLastDay bool) (uint64, bool, error) {
	var mask uint64
	hasLastDay := false

	parts := strings.Split(field, ",")
	for _, rawPart := range parts {
		part := strings.TrimSpace(rawPart)
		if part == "" {
			return 0, false, fmt.Errorf("token kosong di antara koma")
		}
		if allowLastDay && strings.EqualFold(part, "L") {
			hasLastDay = true
			continue
		}

		// Pisahkan step opsional (/S)
		rangePart := part
		step := 1
		if strings.Contains(part, "/") {
			sub := strings.Split(part, "/")
			if len(sub) != 2 || sub[0] == "" || sub[1] == "" {
				return 0, false, fmt.Errorf("format step tidak valid pada %q", part)
			}
			rangePart = sub[0]
			s, err := strconv.Atoi(sub[1])
			if err != nil || s <= 0 {
				return 0, false, fmt.Errorf("nilai step harus angka positif pada %q", part)
			}
			step = s
		}

		var start, end int
		switch {
		case rangePart == "*":
			start = minVal
			end = maxVal
			if isDOW {
				end = 6
			}
		case strings.Contains(rangePart, "-"):
			bounds := strings.Split(rangePart, "-")
			if len(bounds) != 2 {
				return 0, false, fmt.Errorf("format rentang tidak valid pada %q", part)
			}
			var err error
			start, err = parseCronAtom(bounds[0], minVal, maxVal, aliases, isDOW)
			if err != nil {
				return 0, false, err
			}
			end, err = parseCronAtom(bounds[1], minVal, maxVal, aliases, isDOW)
			if err != nil {
				return 0, false, err
			}
			// Izinkan rentang hari kerja sampai Minggu (misal 5-7 atau FRI-SUN)
			if isDOW && end == 0 && strings.EqualFold(bounds[1], "SUN") || (isDOW && bounds[1] == "7") {
				end = 7
			}
			if start > end {
				return 0, false, fmt.Errorf("rentang terbalik (%d-%d)", start, end)
			}
		default:
			val, err := parseCronAtom(rangePart, minVal, maxVal, aliases, isDOW)
			if err != nil {
				return 0, false, err
			}
			start = val
			if strings.Contains(part, "/") {
				end = maxVal
				if isDOW {
					end = 6
				}
			} else {
				end = val
			}
		}

		for v := start; v <= end; v += step {
			bit := v
			if isDOW && bit == 7 {
				bit = 0
			}
			mask |= 1 << uint(bit)
		}
	}

	if mask == 0 && !hasLastDay {
		return 0, false, fmt.Errorf("tidak ada nilai yang cocok")
	}
	return mask, hasLastDay, nil
}

func parseCronAtom(tok string, minVal, maxVal int, aliases map[string]int, isDOW bool) (int, error) {
	upper := strings.ToUpper(strings.TrimSpace(tok))
	if aliases != nil {
		if v, ok := aliases[upper]; ok {
			return v, nil
		}
	}
	n, err := strconv.Atoi(upper)
	if err != nil {
		return 0, fmt.Errorf("nilai %q bukan angka yang sah", tok)
	}
	if n < minVal || n > maxVal {
		return 0, fmt.Errorf("nilai %d di luar rentang %d..%d", n, minVal, maxVal)
	}
	if isDOW && n == 7 {
		return 0, nil
	}
	return n, nil
}

// daysInMonth mengembalikan jumlah hari aktual dalam bulan (memperhitungkan tahun kabisat)
func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// matches memeriksa apakah waktu t cocok terhadap jadwal ter-parse (mengikuti semantik Vixie cron standar)
func (ps *parsedSchedule) matches(t time.Time) bool {
	if (ps.months & (1 << uint(t.Month()))) == 0 {
		return false
	}
	if (ps.hours & (1 << uint(t.Hour()))) == 0 {
		return false
	}
	if (ps.minutes & (1 << uint(t.Minute()))) == 0 {
		return false
	}
	return ps.matchesDay(t)
}

func (ps *parsedSchedule) matchesDay(t time.Time) bool {
	domMatch := (ps.days&(1<<uint(t.Day()))) != 0 || (ps.lastDay && t.Day() == daysInMonth(t.Year(), t.Month()))
	dowMatch := (ps.dows & (1 << uint(t.Weekday()))) != 0

	// Semantik standar Vixie cron: bila tanggal DAN hari sama-sama dibatasi (bukan *), salah satu cocok sudah memenuhi
	if ps.domRestricted && ps.dowRestricted {
		return domMatch || dowMatch
	}
	return domMatch && dowMatch
}

// matchCronExpression mencocokkan waktu sekarang terhadap ekspresi 5-kolom standar
func matchCronExpression(fields []string, t time.Time) bool {
	ps, err := parseSchedule(strings.Join(fields, " "))
	if err != nil {
		return false
	}
	return ps.matches(t)
}

// NextRunAt menghitung waktu eksekusi terdekat berikutnya setelah waktu acuan from
func NextRunAt(spec string, from time.Time) (time.Time, error) {
	ps, err := parseSchedule(spec)
	if err != nil {
		return time.Time{}, err
	}

	// Mulai pencarian dari awal menit berikutnya dalam zona waktu yang sama
	cur := from.Truncate(time.Minute).Add(time.Minute)
	limit := cur.AddDate(4, 0, 0)

	for cur.Before(limit) {
		if (ps.months & (1 << uint(cur.Month()))) == 0 {
			cur = time.Date(cur.Year(), cur.Month()+1, 1, 0, 0, 0, 0, cur.Location())
			continue
		}
		if !ps.matchesDay(cur) {
			cur = time.Date(cur.Year(), cur.Month(), cur.Day()+1, 0, 0, 0, 0, cur.Location())
			continue
		}
		if (ps.hours & (1 << uint(cur.Hour()))) == 0 {
			cur = time.Date(cur.Year(), cur.Month(), cur.Day(), cur.Hour()+1, 0, 0, 0, cur.Location())
			continue
		}
		if (ps.minutes & (1 << uint(cur.Minute()))) == 0 {
			cur = cur.Add(time.Minute)
			continue
		}
		return cur, nil
	}
	return time.Time{}, fmt.Errorf("tidak ditemukan jadwal cocok dalam 4 tahun ke depan")
}
