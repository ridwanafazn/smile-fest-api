package cloudinary

import (
	"context"
	"errors"
	"mime/multipart"
	"os"
	"strings"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

// UploadImage mengunggah file gambar ke Cloudinary dan mengembalikan URL
// (HTTPS) yang sudah dilengkapi transformasi optimasi delivery.
func UploadImage(ctx context.Context, file multipart.File, filename string) (string, error) {
	// Membaca credential dari environment variable (.env)
	cldURL := os.Getenv("CLOUDINARY_URL")
	if cldURL == "" {
		return "", errors.New("CLOUDINARY_URL belum diatur di .env")
	}

	// Inisialisasi client Cloudinary
	cld, err := cloudinary.NewFromURL(cldURL)
	if err != nil {
		return "", err
	}

	// Proses Upload
	// Menggunakan param public_id agar nama file di cloud sesuai dengan Order ID
	resp, err := cld.Upload.Upload(ctx, file, uploader.UploadParams{
		Folder:   "smile-fest/payment-proofs",
		PublicID: filename,
	})

	if err != nil {
		return "", err
	}

	// Kembalikan URL gambar yang sudah dikompresi otomatis saat disajikan.
	return addDeliveryTransform(resp.SecureURL), nil
}

// addDeliveryTransform menyisipkan transformasi optimasi ke URL Cloudinary
// agar gambar ter-kompres otomatis saat delivery (tanpa mengubah file asli):
// f_auto (format terbaik), q_auto (kualitas adaptive), w_1000 (maks lebar),
// c_limit (jangan memperbesar).
func addDeliveryTransform(secureURL string) string {
	const marker = "/image/upload/"
	idx := strings.Index(secureURL, marker)
	if idx == -1 {
		return secureURL
	}
	insertAt := idx + len(marker)
	return secureURL[:insertAt] + "f_auto,q_auto,w_1000,c_limit/" + secureURL[insertAt:]
}
