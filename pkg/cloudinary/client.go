package cloudinary

import (
	"context"
	"errors"
	"mime/multipart"
	"os"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

// UploadImage mengunggah file gambar ke Cloudinary dan mengembalikan URL (HTTPS)
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

	// Kembalikan URL gambar yang sudah jadi
	return resp.SecureURL, nil
}
