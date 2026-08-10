# MHRS Randevu Botu

Windows üzerinde çalışan, MHRS'deki uygun hastane randevularını belirli
aralıklarla kontrol edip terminalde bildiren açık kaynak Go uygulaması.

> [!IMPORTANT]
> Bu proje yalnızca randevu sorgular ve bildirim verir. Randevu oluşturmaz,
> onaylamaz, değiştirmez veya iptal etmez.

Bu proje Sağlık Bakanlığı veya MHRS tarafından geliştirilmemiştir. MHRS'nin
belgelenmemiş web servislerini kullandığı için servis değişikliklerinden
etkilenebilir.

## Özellikler

- e-Devlet veya e-Nabız girişini kullanıcıya görünür tarayıcıda yaptırır.
- Giriş alanlarını, e-Devlet parolasını veya e-Nabız parolasını okumaz.
- MHRS oturumunu Windows DPAPI ile şifreleyerek sonraki çalıştırmalarda yeniden
  kullanır.
- İl için güncel poliklinik listesini MHRS'den getirir.
- Tek seferlik veya belirli aralıklarla randevu kontrolü yapar.
- Randevu bulunduğunda ayrıntıları terminale yazar ve terminal zilini çalar.
- İsteğe bağlı olarak randevu ayrıntılarını Telegram mesajıyla bildirir.
- Tarihleri Türkiye saatiyle, Türkçe ay adı ve `TSİ` ekiyle gösterir.
- Durumları renkli terminal çıktısıyla birbirinden ayırır.
- CAPTCHA, SMS doğrulaması veya oran sınırlaması aşmaya çalışmaz.

## Gereksinimler

- Windows 10 veya Windows 11
- Go 1.26 veya üzeri
- Chromium, Google Chrome veya Microsoft Edge

## Kurulum

```powershell
git clone https://github.com/berkeserce/mhrs-randevu-botu.git
cd mhrs-randevu-botu
go mod download
```

## Kullanım

En kolay kullanım için parametre vermeden çalıştırın:

```powershell
go run ./cmd/mhrs-randevu-botu
```

Uygulama terminalde sırasıyla şunları sorar:

1. İl plaka kodu
2. Kaç günlük randevu penceresi istendiği (1-16 gün)
3. Tek sorgu veya sürekli takip seçimi
4. Sürekli takipte kontrol aralığı

Ardından kayıtlı oturumu kullanır veya gerekirse tarayıcı girişini açar ve
poliklinik seçimine geçer.

### Parametrelerle kullanım

İlk denemeyi tek sorguyla yapmak için il plaka kodunu verebilirsiniz:

```powershell
go run ./cmd/mhrs-randevu-botu -once -il 34
```

Uygulama ilk çalıştırmada görünür bir tarayıcı açar. e-Devlet veya e-Nabız
girişini tarayıcıda tamamladıktan sonra poliklinik arama alanına örneğin
`göz` yazıp listeden seçim yapın.

Seçim sırasında gösterilen sayısal poliklinik ID'sini sonraki çalıştırmalarda
`-klinik` parametresine verebilirsiniz. ID verilmezse seçim ekranı yeniden
açılır:

```powershell
go run ./cmd/mhrs-randevu-botu -il 34
```

Varsayılan olarak yalnızca takip başladığı andan sonraki 3 gün içine düşen
randevular bildirilir. Kullanıcı 1 ile 16 gün arasında sabit bir randevu ve
takip penceresi seçebilir. Örneğin önümüzdeki 5 gün içindeki randevuları aramak
için:

```powershell
go run ./cmd/mhrs-randevu-botu -il 34 -gun 5
```

Bu örnekte 5 günden sonraki randevular bildirilmez. Program uygun randevu
bulamazsa 5 gün boyunca belirlenen aralıkla sorgulamaya devam eder; pencere
sonunda sonucu yazıp kapanır. `-once` verilirse pencere yine uygulanır ancak
yalnızca tek sorgu yapılır.

Uzun takip sırasında MHRS JWT'sinin süresi dolarsa uygulama takip penceresini
iptal etmez; tarayıcıyı yeniden açarak kullanıcıdan manuel e-Devlet/e-Nabız
girişini yenilemesini ister. Giriş zaman aşımında tamamlanmazsa güvenli biçimde
durur.

Varsayılan kontrol aralığı 5 dakikadır. Farklı bir aralık için:

```powershell
go run ./cmd/mhrs-randevu-botu -il 34 -interval 10m
```

Programı durdurmak için `Ctrl+C` kullanın. Sürekli sorgulamada bir dakikanın
altındaki aralıklara izin verilmez.

### Terminal renkleri ve saat

Tüm oturum, kontrol ve randevu zamanları `10 Ağustos 2026 17:13:59 TSİ`
biçiminde gösterilir. Renkler yalnızca etkileşimli terminalde etkinleştirilir;
dosyaya veya pipe'a yönlendirilen çıktıya ANSI kodu yazılmaz.

Renkleri elle kapatmak için:

```powershell
go run ./cmd/mhrs-randevu-botu -no-color
```

Standart `NO_COLOR` ortam değişkeni de desteklenir.

### Telegram bildirimi

Telegram bildirimi isteğe bağlıdır. Ayarlar, Git tarafından yok sayılan yerel
`.env` dosyasından otomatik okunur. `.env` dosyasını public repoya veya başka bir
kişiye göndermeyin.

1. Telegram'da resmî `@BotFather` hesabını açın, `/newbot` komutuyla bot oluşturun
   ve verilen tokenı güvenli tutun.
2. Oluşturduğunuz botun sohbetini açıp `/start` gönderin. Botlar, kullanıcı sohbeti
   başlatmadan kullanıcıya mesaj gönderemez.
3. `.env.example` dosyasını `.env` adıyla kopyalayın:

```powershell
Copy-Item .env.example .env
```

4. `.env` dosyasını açıp `TELEGRAM_BOT_TOKEN=` satırındaki eşittir işaretinin
   sağına BotFather'ın verdiği tokenı yazın.
5. Bot sohbetine `/start` gönderdikten sonra chat ID'yi bulun:

```powershell
go run ./cmd/mhrs-randevu-botu -telegram-chat-id
```

6. Çıkan sayıyı `.env` içindeki `TELEGRAM_CHAT_ID=` satırına yazıp test bildirimi
   gönderin:

```powershell
go run ./cmd/mhrs-randevu-botu -telegram-test
```

`Telegram test bildirimi gonderildi.` mesajını ve Telegram bildirimini gördükten
sonra uygulamayı normal biçimde çalıştırabilirsiniz:

```powershell
go run ./cmd/mhrs-randevu-botu
```

İki ortam değişkeni ayarlıysa uygun randevu bulunduğunda en fazla ilk beş
seçeneğin hastane, poliklinik, hekim, muayene yeri ve zaman bilgisi Telegram'a
gönderilir. Telegram isteği başarısız olursa uygulama hatayı terminalde bildirir;
konsoldaki randevu sonucu yine gösterilir. İşletim sistemi ortam değişkenleri de
desteklenir ve ayarlanmışlarsa `.env` içindeki aynı adlı değerlerden önceliklidir.

### Oturum yönetimi

JWT düz metin veya `.env` dosyası olarak saklanmaz. Windows kullanıcı hesabına
bağlı DPAPI şifrelemesiyle şu konuma yazılır:

```text
%LocalAppData%\mhrs-randevu-botu\session.bin
```

Tokenin süresi dolmuşsa uygulama kaydı siler ve tarayıcı girişini yeniden açar.
Yeni oturumu zorlamak için:

```powershell
go run ./cmd/mhrs-randevu-botu -once -il 34 -refresh-session
```

Kaydı silmek için:

```powershell
go run ./cmd/mhrs-randevu-botu -clear-session
```

### Tarayıcı yolu

Uygulama Chromium, Chrome ve Edge'in standart Windows kurulum konumlarını
otomatik kontrol eder. Bulamazsa yolu elle verin:

```powershell
go run ./cmd/mhrs-randevu-botu -once -il 34 -chromium-path "C:\Program Files\Chromium\Application\chrome.exe"
```

Diğer seçenekleri görmek için:

```powershell
go run ./cmd/mhrs-randevu-botu -help
```

## Güvenlik ve gizlilik

- Uygulamayı yalnızca kendi MHRS hesabınızla kullanın.
- T.C. kimlik numarası, telefon numarası, parola ve SMS kodu uygulama tarafından
  istenmez veya saklanmaz.
- JWT hassas veridir. Şifreli oturum dosyasını paylaşmayın.cd 
- Chromium giriş için geçici, ayrı bir profil kullanır; işlem bitince profil
  silinir ve pencere kapanır.
- Proje içine `.env` dosyaları, JWT veya kişisel çıktı eklemeyin.
- Telegram bot tokenını parola gibi koruyun. Telegram bildirimi hastane,
  poliklinik, hekim ve randevu zamanı gibi sağlıkla ilişkili bilgileri Telegram'a
  aktarır; bu özelliği yalnızca bunu kabul ediyorsanız etkinleştirin.

## Proje kapsamı

Uygulamanın MHRS istemcisi yalnızca poliklinik kataloğunu ve uygun randevuları
okur. Otomatik randevu alma özellikle kapsam dışıdır. Bu sınırı değiştiren
katkılar kabul edilmez.

## Geliştirme

```powershell
go fmt ./...
go vet ./...
go test -race ./...
```

MHRS servisleri public ve sürümlenmiş bir API sağlamadığı için ağ katmanındaki
birim testleri yerel HTTP test sunucularıyla çalışır; gerçek MHRS hesabına
bağlanmaz.

## Sorumluluk reddi

Bu yazılım eğitim ve kişisel bildirim amacıyla sunulur. Kullanıcı, MHRS kullanım
koşullarına ve yürürlükteki kurallara uymaktan sorumludur. Servisi gereksiz
yükleyecek kullanım yapmayın.

## Lisans

[MIT](LICENSE)
