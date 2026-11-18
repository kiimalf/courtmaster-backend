# CourtMaster Backend API

Backend API untuk sistem reservasi gelanggang olahraga dan sesi sparring/mabar komunitas. Dibangun dengan Go (Fiber v2), arsitektur poliglota (PostgreSQL + MongoDB), serta *event-driven background worker* (Redis + Asynq + Transactional Outbox).

---

## Tech Stack

- **Language & Framework:** Go 1.22+, [Fiber v2](https://github.com/gofiber/fiber)
- **Relational DB:** PostgreSQL 16 (via `pgx/v5` pool)
- **Document DB:** MongoDB 7.0 (Review & rating agregasi)
- **Task Queue & Cache:** Redis 7 + [Asynq](https://github.com/hibiken/asynq)
- **Auth:** JWT (Access Token + Refresh Token via DB)
- **Documentation:** Swagger / OpenAPI (`swaggo`)

---

## Panduan Instalasi & Menjalankan

### 1. Prasyarat
- Go 1.22 atau lebih baru
- Docker & Docker Compose (opsional, disarankan)

### 2. Konfigurasi Environment
Salin file konfigurasi contoh:
```bash
cp .env.example .env
```
Sesuaikan nilai variabel di `.env` jika diperlukan (minimal `JWT_SECRET` dengan panjang >= 32 karakter).

---

### Opsi A: Menggunakan Docker Compose (Direkomendasikan)
Menjalankan PostgreSQL, MongoDB, Redis, API, dan Worker secara otomatis:

```bash
# Menyalakan seluruh service
docker compose up -d --build

# Melihat log
docker compose logs -f api

# Menghentikan service
docker compose down
```
*Catatan: File migrasi SQL pada folder `./migrations` otomatis dieksekusi saat inisialisasi PostgreSQL container.*

---

### Opsi B: Menjalankan Manual / Lokal

1. **Jalankan Database** (PostgreSQL, MongoDB, Redis aktif di host lokal).
2. **Jalankan Migrasi SQL** ke PostgreSQL secara berurutan:
   ```bash
   psql -U courtmaster -d courtmaster_db -f migrations/001_roles_and_permissions.sql
   psql -U courtmaster -d courtmaster_db -f migrations/002_users.sql
   psql -U courtmaster -d courtmaster_db -f migrations/003_courts_and_slots.sql
   psql -U courtmaster -d courtmaster_db -f migrations/004_refresh_tokens.sql
   psql -U courtmaster -d courtmaster_db -f migrations/005_bookings_and_sparrings.sql
   psql -U courtmaster -d courtmaster_db -f migrations/006_outbox_and_notifications.sql
   ```
3. **Jalankan Aplikasi:**
   ```bash
   # Menjalankan API & Worker sekaligus
   go run . -role=all

   # ATAU di terminal terpisah:
   go run . -role=api      # Server HTTP Fiber (:8080)
   go run . -role=worker   # Asynq Worker & Outbox Relay
   ```

---

## Akun Bawaan (Default Seed Data)

Sistem sudah dilengkapi seed akun bawaan untuk pengujian:

| Role | Username | Password | Deskripsi |
| :--- | :--- | :--- | :--- |
| **Admin** | `admin` | `AdminMaster2026` | Akses penuh (manajemen sistem & antrean) |
| **Manager** | `manager_central` | `ManagerPadel123` | Kelola gelanggang & moderasi ulasan |
| **Player** | `player_budi` | `BudiMainPadel2026` | Booking gelanggang & sesi sparring |
| **Player** | `player_sari` | `SariBadminton2026` | Booking gelanggang & sesi sparring |

---

## Daftar Route / Endpoints API

Base URL: `http://localhost:8080/api/v1`

### 1. Healthcheck & Dokumentasi
| Method | Endpoint | Auth / Permission | Keterangan |
| :--- | :--- | :--- | :--- |
| `GET` | `/health` | Public | Cek koneksi PostgreSQL & MongoDB |
| `GET` | `/swagger/*` | Public | Dokumentasi interaktif Swagger UI |

### 2. Autentikasi (`/auth`)
| Method | Endpoint | Auth / Permission | Keterangan |
| :--- | :--- | :--- | :--- |
| `POST` | `/auth/register` | Public | Registrasi akun baru |
| `POST` | `/auth/login` | Public (Rate Limited) | Login & generate Access/Refresh token |
| `POST` | `/auth/refresh` | Public | Refresh token JWT yang kedaluwarsa |
| `POST` | `/auth/logout` | Bearer Token | Revoke refresh token |
| `GET` | `/auth/me` | Bearer Token | Ambil profil user yang sedang login |

### 3. Gelanggang Olahraga (`/courts`)
| Method | Endpoint | Auth / Permission | Keterangan |
| :--- | :--- | :--- | :--- |
| `GET` | `/courts` | Public | Daftar seluruh gelanggang |
| `GET` | `/courts/:id` | Public | Detail spesifik gelanggang |
| `GET` | `/courts/:id/slots` | Public | Daftar slot ketersediaan jadwal |
| `POST` | `/courts` | `court:create` | Tambah gelanggang baru |
| `PATCH`| `/courts/:id/rate` | `court:manage` | Perbarui tarif per jam |
| `PATCH`| `/courts/:id` | `court:manage` | Perbarui informasi gelanggang |

### 4. Ulasan Gelanggang (`/courts/:id/reviews` & `/reviews`)
*Penyimpanan ulasan menggunakan MongoDB.*

| Method | Endpoint | Auth / Permission | Keterangan |
| :--- | :--- | :--- | :--- |
| `GET` | `/courts/:id/reviews` | Public | Daftar ulasan gelanggang |
| `GET` | `/courts/:id/reviews/summary` | Public | Agregasi rating & ringkasan |
| `POST` | `/courts/:id/reviews` | `review:create` | Berikan ulasan gelanggang |
| `POST` | `/courts/:id/reviews/:reviewId/helpful` | Bearer Token | Vote review membantu (+1) |
| `POST` | `/courts/:id/reviews/:reviewId/replies` | Bearer Token | Tambah balasan/tanggapan review |
| `POST` | `/reviews/:id/helpful` | Bearer Token | Alias vote helpful review |
| `POST` | `/reviews/:id/replies` | Bearer Token | Alias balas review |

### 5. Reservasi (`/bookings`)
| Method | Endpoint | Auth / Permission | Keterangan |
| :--- | :--- | :--- | :--- |
| `POST` | `/bookings` | `booking:create` | Booking slot lapangan (ACID + Outbox) |
| `GET` | `/bookings` | `booking:read` | Daftar reservasi (Keyset Pagination) |
| `GET` | `/bookings/my` | `booking:read` | Riwayat reservasi milik sendiri |
| `POST` | `/bookings/swap` | `booking:update` | Penukaran jadwal booking |
| `GET` | `/bookings/export` | `booking:export` | Ekspor data booking |

### 6. Sparring / Mabar (`/sparrings`)
| Method | Endpoint | Auth / Permission | Keterangan |
| :--- | :--- | :--- | :--- |
| `GET` | `/sparrings` | Public | Daftar sesi sparring terbuka |
| `POST` | `/sparrings` | `sparring:create` | Buat sesi sparring/mabar baru |
| `POST` | `/sparrings/:id/join` | `sparring:join` | Bergabung ke sesi sparring |

### 7. Notifikasi & Monitoring Antrean
| Method | Endpoint | Auth / Permission | Keterangan |
| :--- | :--- | :--- | :--- |
| `GET` | `/notifications` | Bearer Token | Daftar notifikasi sistem |
| `GET` | `/notifications/mine` | Bearer Token | Riwayat notifikasi user |
| `POST` | `/notifications/outbox/retry` | `outbox:manage` | Retry event outbox yang belum terkirim |
| `GET` | `/admin/queue` | `queue:monitor` | Cek status & metrik antrean Asynq/Redis |

---

## Menjalankan Pengujian (Testing)

```bash
# Menjalankan seluruh unit & integration test
go test -v ./...
```
