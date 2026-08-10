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
