package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/berkeserce/mhrs-randevu-botu/internal/browserauth"
	"github.com/berkeserce/mhrs-randevu-botu/internal/mhrs"
	"github.com/berkeserce/mhrs-randevu-botu/internal/sessioncache"
	"golang.org/x/term"
)

const maxAppointmentWindowDays = 16

func main() {
	chromiumPath := flag.String("chromium-path", "", "chrome.exe veya chromium calistirilabilir dosyasinin yolu")
	browserTimeout := flag.Duration("browser-timeout", 10*time.Minute, "manuel tarayici girisi zaman asimi")
	refreshSession := flag.Bool("refresh-session", false, "kayitli JWT yerine Chromium ile yeni oturum al")
	clearSession := flag.Bool("clear-session", false, "kayitli sifreli MHRS oturumunu sil ve cik")
	once := flag.Bool("once", false, "randevuyu yalnizca bir kez kontrol et")
	interval := flag.Duration("interval", 5*time.Minute, "randevu kontrol araligi (en az 1 dakika)")
	days := flag.Int("gun", 3, "randevu tarihi ve takip suresi (1-16 gun)")
	cityID := flag.Int64("il", -1, "MHRS il ID")
	districtID := flag.Int64("ilce", -1, "MHRS ilce ID (-1: tumu)")
	clinicID := flag.Int64("klinik", -1, "MHRS klinik ID (verilmezse listeden secilir)")
	institutionID := flag.Int64("kurum", -1, "MHRS kurum ID (-1: tumu)")
	doctorID := flag.Int64("hekim", -1, "MHRS hekim ID (-1: tumu)")
	examinationID := flag.Int64("muayene-yeri", -1, "MHRS muayene yeri ID (-1: tumu)")
	gender := flag.String("cinsiyet", "F", "MHRS cinsiyet filtresi: F (varsayilan/farketmez) veya M")
	flag.Parse()

	if *clearSession {
		if err := sessioncache.Remove(); err != nil {
			exitWithError(err)
		}
		fmt.Println("Kayitli MHRS oturumu silindi.")
		return
	}
	if flag.NFlag() == 0 {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			exitWithError(errors.New("parametresiz kullanim icin etkilesimli terminal gerekli; secenekler icin -help kullanin"))
		}
		config, err := promptInteractiveConfig(bufio.NewReader(os.Stdin), os.Stdout)
		if err != nil {
			exitWithError(err)
		}
		*cityID = config.cityID
		*days = config.days
		*once = config.once
		*interval = config.interval
	}

	criteria := mhrs.SearchCriteria{
		ActionID: 200, ActionIDs: []int64{200, 204, 218}, Gender: strings.ToUpper(strings.TrimSpace(*gender)),
		DoctorID: *doctorID, CityID: *cityID, DistrictID: *districtID,
		ClinicID: *clinicID, InstitutionID: *institutionID, ExaminationID: *examinationID,
	}
	if err := runWatch(*chromiumPath, *browserTimeout, *interval, *once, *refreshSession, *days, criteria); err != nil {
		exitWithError(err)
	}
}

type interactiveConfig struct {
	cityID   int64
	days     int
	once     bool
	interval time.Duration
}

func promptInteractiveConfig(reader *bufio.Reader, writer io.Writer) (interactiveConfig, error) {
	fmt.Fprintln(writer, "============================================================")
	fmt.Fprintln(writer, "MHRS Randevu Botu")
	fmt.Fprintln(writer, "Yalnizca uygun randevuyu sorgular ve bildirir; randevu almaz.")
	fmt.Fprintln(writer, "============================================================")

	city, err := promptNumber(reader, writer, "Il plaka kodu (1-81): ", 1, 81, 0)
	if err != nil {
		return interactiveConfig{}, err
	}
	days, err := promptNumber(reader, writer, "Kac gun icindeki randevular aransin? (1-16) [3]: ", 1, maxAppointmentWindowDays, 3)
	if err != nil {
		return interactiveConfig{}, err
	}
	mode, err := promptNumber(reader, writer, "Kontrol tipi: 1=Tek sorgu, 2=Suresi dolana kadar takip [2]: ", 1, 2, 2)
	if err != nil {
		return interactiveConfig{}, err
	}

	config := interactiveConfig{cityID: int64(city), days: days, once: mode == 1, interval: 5 * time.Minute}
	if !config.once {
		minutes, err := promptNumber(reader, writer, "Kontrol araligi kac dakika olsun? [5]: ", 1, 24*60, 5)
		if err != nil {
			return interactiveConfig{}, err
		}
		config.interval = time.Duration(minutes) * time.Minute
	}
	modeText := "tek sorgu"
	if !config.once {
		modeText = fmt.Sprintf("surekli takip, %s aralikla", config.interval)
	}
	fmt.Fprintf(writer, "\nSecimler: il=%d, pencere=%d gun, mod=%s\n\n", config.cityID, config.days, modeText)
	return config, nil
}

func promptNumber(reader *bufio.Reader, writer io.Writer, prompt string, minValue, maxValue, defaultValue int) (int, error) {
	for {
		value, err := readPrompt(reader, writer, prompt)
		if err != nil {
			return 0, err
		}
		if value == "" && defaultValue >= minValue && defaultValue <= maxValue {
			return defaultValue, nil
		}
		number, err := strconv.Atoi(value)
		if err == nil && number >= minValue && number <= maxValue {
			return number, nil
		}
		fmt.Fprintf(writer, "Lutfen %d ile %d arasinda bir sayi girin.\n", minValue, maxValue)
	}
}

func exitWithError(err error) {
	fmt.Fprintln(os.Stderr, "Hata:", err)
	os.Exit(1)
}

func acquireBrowserToken(parent context.Context, preferredExecutable string, timeout time.Duration, forceRefresh bool) (string, browserauth.JWTInfo, error) {
	if timeout <= 0 {
		return "", browserauth.JWTInfo{}, errors.New("browser-timeout sifirdan buyuk olmali")
	}
	if !forceRefresh {
		if token, err := sessioncache.Load(); err == nil {
			info, parseErr := browserauth.ParseJWT(token, time.Now().Add(2*time.Minute))
			if parseErr == nil {
				fmt.Printf("Kayitli sifreli MHRS oturumu kullaniliyor. Oturum sonu: %s\n", info.ExpiresAt.Local().Format(time.RFC3339))
				return token, info, nil
			}
			_ = sessioncache.Remove()
			fmt.Println("Kayitli MHRS oturumunun suresi dolmus; yeni giris gerekiyor.")
		} else if !errors.Is(err, sessioncache.ErrNotFound) {
			_ = sessioncache.Remove()
			fmt.Printf("Kayitli MHRS oturumu kullanilamadi (%v); yeni giris gerekiyor.\n", err)
		}
	}

	executablePath, err := browserauth.FindExecutable(preferredExecutable)
	if err != nil {
		return "", browserauth.JWTInfo{}, err
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	fmt.Println("Chromium aciliyor. e-Devlet veya e-Nabiz girisini acilan pencerede kendiniz tamamlayin.")
	fmt.Println("Uygulama giris alanlarini okumaz; MHRS'ye donuldugunde yalnizca MHRS JWT'sini alir.")
	token, err := browserauth.Login(ctx, executablePath)
	if err != nil {
		return "", browserauth.JWTInfo{}, err
	}
	info, err := browserauth.ParseJWT(token, time.Now())
	if err != nil {
		return "", browserauth.JWTInfo{}, err
	}
	if err := sessioncache.Save(token); err != nil {
		fmt.Printf("Uyari: JWT sifreli onbellege kaydedilemedi: %v\n", err)
	} else {
		fmt.Println("MHRS oturumu Windows hesabina bagli sifreli onbellege kaydedildi.")
	}
	return token, info, nil
}

func runWatch(executablePath string, browserTimeout, interval time.Duration, once, refreshSession bool, days int, criteria mhrs.SearchCriteria) error {
	if err := validateSearchCriteria(criteria, interval, once, days); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	token, info, err := acquireBrowserToken(ctx, executablePath, browserTimeout, refreshSession)
	if err != nil {
		return err
	}
	fmt.Printf("MHRS girisi basarili. Oturum sonu: %s\n", info.ExpiresAt.Local().Format(time.RFC3339))
	client := mhrs.NewAuthenticatedClient(token)
	reader := bufio.NewReader(os.Stdin)
	for attempt := 0; ; attempt++ {
		criteria, err = selectClinic(ctx, client, criteria, reader)
		if err == nil {
			break
		}
		if !errors.Is(err, mhrs.ErrSessionExpired) || attempt > 0 {
			discardExpiredSession(err)
			return err
		}
		fmt.Println("Kayitli MHRS oturumu sunucu tarafinda sona ermis; yeniden giris gerekiyor.")
		_ = sessioncache.Remove()
		token, info, err = acquireBrowserToken(ctx, executablePath, browserTimeout, true)
		if err != nil {
			return err
		}
		fmt.Printf("MHRS girisi yenilendi. Oturum sonu: %s\n", info.ExpiresAt.Local().Format(time.RFC3339))
		client = mhrs.NewAuthenticatedClient(token)
	}

	watchStartedAt := time.Now()
	watchDeadline := watchStartedAt.Add(time.Duration(days) * 24 * time.Hour)
	fmt.Printf("Randevu penceresi: %s - %s (secilen sure: %d gun)\n",
		watchStartedAt.Format("02.01.2006 15:04"), watchDeadline.Format("02.01.2006 15:04"), days)

	for {
		checkedAt := time.Now()
		if !checkedAt.Before(watchDeadline) {
			printFinalNoAppointment(days, watchStartedAt, watchDeadline)
			return nil
		}
		items, err := client.SearchAppointments(ctx, criteria)
		if err != nil {
			if errors.Is(err, mhrs.ErrSessionExpired) {
				_ = sessioncache.Remove()
				fmt.Println("MHRS oturumu sona erdi; takip suresi devam ettigi icin yeniden giris gerekiyor.")
				token, info, err = acquireBrowserToken(ctx, executablePath, browserTimeout, true)
				if err != nil {
					return err
				}
				fmt.Printf("MHRS girisi yenilendi. Oturum sonu: %s\n", info.ExpiresAt.Local().Format(time.RFC3339))
				client = mhrs.NewAuthenticatedClient(token)
				continue
			}
			discardExpiredSession(err)
			return err
		}
		eligible := filterAvailabilityByWindow(items, checkedAt, watchDeadline)
		if len(eligible) > 0 {
			printAvailability(eligible, checkedAt)
			return nil
		}
		fmt.Printf("[%s] Onumuzdeki %d gun icinde uygun randevu bulunamadi.\n", checkedAt.Format("02.01.2006 15:04:05"), days)
		if once {
			return nil
		}
		wait := interval
		if remaining := time.Until(watchDeadline); remaining < wait {
			wait = remaining
		}
		next := time.Now().Add(wait)
		fmt.Printf("Sonraki kontrol: %s (cikmak icin Ctrl+C)\n", next.Format("02.01.2006 15:04:05"))
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

func filterAvailabilityByWindow(items []mhrs.Availability, now, deadline time.Time) []mhrs.Availability {
	result := make([]mhrs.Availability, 0, len(items))
	for _, item := range items {
		appointment, err := parseAppointmentTime(item.StartTime, now.Location())
		if err != nil || appointment.Before(now) || appointment.After(deadline) {
			continue
		}
		result = append(result, item)
	}
	return result
}

func parseAppointmentTime(value string, location *time.Location) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"02.01.2006 15:04", "02.01.2006 15:04:05", time.RFC3339} {
		parsed, err := time.ParseInLocation(layout, value, location)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("randevu zamani okunamadi: %q", value)
}

func printFinalNoAppointment(days int, startedAt, deadline time.Time) {
	fmt.Println("============================================================")
	fmt.Printf("TAKIP SURESI DOLDU: %s - %s\n", startedAt.Format("02.01.2006 15:04"), deadline.Format("02.01.2006 15:04"))
	fmt.Printf("Onumuzdeki %d gun icinde uygun randevu bulunamadi.\n", days)
	fmt.Println("============================================================")
}

func discardExpiredSession(err error) {
	if errors.Is(err, mhrs.ErrSessionExpired) {
		_ = sessioncache.Remove()
	}
}

func validateSearchCriteria(criteria mhrs.SearchCriteria, interval time.Duration, once bool, days int) error {
	if criteria.CityID <= 0 {
		return errors.New("-il pozitif bir MHRS il ID olmali")
	}
	if criteria.Gender != "M" && criteria.Gender != "F" {
		return errors.New("-cinsiyet M veya F olmali")
	}
	if days < 1 || days > maxAppointmentWindowDays {
		return fmt.Errorf("-gun 1 ile %d arasinda olmali", maxAppointmentWindowDays)
	}
	if !once && interval < time.Minute {
		return errors.New("-interval en az 1 dakika olmali")
	}
	return nil
}

func selectClinic(ctx context.Context, client *mhrs.Client, criteria mhrs.SearchCriteria, reader *bufio.Reader) (mhrs.SearchCriteria, error) {
	clinics, err := client.ListClinics(ctx, criteria)
	if err != nil {
		return criteria, fmt.Errorf("guncel poliklinik listesi alinamadi: %w", err)
	}
	if len(clinics) == 0 {
		return criteria, errors.New("MHRS bu il ve secimler icin poliklinik dondurmedi")
	}
	if criteria.ClinicID > 0 {
		for _, clinic := range clinics {
			if clinic.ID == criteria.ClinicID {
				fmt.Printf("Poliklinik dogrulandi: %s (ID: %d)\n", clinic.Name, clinic.ID)
				return criteria, nil
			}
		}
		return criteria, fmt.Errorf("-klinik %d bu il icin guncel MHRS listesinde yok; -klinik parametresini kaldirip listeden secin", criteria.ClinicID)
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return criteria, errors.New("-klinik verilmedi; etkilesimli poliklinik secimi icin terminal gerekli")
	}

	for {
		query, err := readLine(reader, "Poliklinik ara (ornek: goz, kardiyoloji): ")
		if err != nil {
			return criteria, err
		}
		if len([]rune(strings.TrimSpace(query))) < 2 {
			fmt.Println("En az iki karakter yazin.")
			continue
		}
		matches := filterClinics(clinics, query)
		if len(matches) == 0 {
			fmt.Println("Eslesen poliklinik bulunamadi; baska bir ifade deneyin.")
			continue
		}
		if len(matches) > 25 {
			fmt.Printf("%d sonuc var; aramayi biraz daha daraltin.\n", len(matches))
			continue
		}
		for index, clinic := range matches {
			fmt.Printf("%2d. %s (ID: %d)\n", index+1, clinic.Name, clinic.ID)
		}
		choice, err := readLine(reader, "Secim numarasi: ")
		if err != nil {
			return criteria, err
		}
		var selected int
		if _, err := fmt.Sscan(choice, &selected); err != nil || selected < 1 || selected > len(matches) {
			fmt.Println("Gecerli bir secim numarasi girin.")
			continue
		}
		criteria.ClinicID = matches[selected-1].ID
		fmt.Printf("Secilen poliklinik: %s (ID: %d)\n", matches[selected-1].Name, criteria.ClinicID)
		return criteria, nil
	}
}

func filterClinics(clinics []mhrs.ClinicOption, query string) []mhrs.ClinicOption {
	normalizedQuery := normalizeSearchText(query)
	result := make([]mhrs.ClinicOption, 0)
	for _, clinic := range clinics {
		if strings.Contains(normalizeSearchText(clinic.Name), normalizedQuery) {
			result = append(result, clinic)
		}
	}
	return result
}

func normalizeSearchText(value string) string {
	value = strings.NewReplacer("İ", "i", "I", "i", "ı", "i").Replace(value)
	return strings.ToLower(strings.TrimSpace(value))
}

func printAvailability(items []mhrs.Availability, checkedAt time.Time) {
	fmt.Print("\a")
	fmt.Println("============================================================")
	fmt.Printf("RANDEVU BULUNDU! Kontrol zamani: %s\n", checkedAt.Format("02.01.2006 15:04:05"))
	for index, item := range items {
		fmt.Printf("\n%d. secenek\n", index+1)
		fmt.Printf("   Hastane: %s\n", displayValue(item.InstitutionName))
		fmt.Printf("   Klinik: %s\n", displayValue(item.ClinicName))
		fmt.Printf("   Hekim: %s\n", displayValue(item.DoctorName))
		fmt.Printf("   Muayene yeri: %s\n", displayValue(item.ExaminationName))
		fmt.Printf("   Zaman: %s\n", displayValue(item.StartTime))
	}
	fmt.Println("\nHemen kontrol et: https://mhrs.gov.tr/vatandas/#/")
	fmt.Println("============================================================")
}

func displayValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "Bilinmiyor"
	}
	return value
}

func readLine(reader *bufio.Reader, prompt string) (string, error) {
	return readPrompt(reader, os.Stdout, prompt)
}

func readPrompt(reader *bufio.Reader, writer io.Writer, prompt string) (string, error) {
	fmt.Fprint(writer, prompt)
	value, err := reader.ReadString('\n')
	if err != nil && len(value) == 0 {
		return "", err
	}
	return strings.TrimSpace(value), nil
}
