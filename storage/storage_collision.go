package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"

	pb "github.com/FedorZaytsev/FedorMemes"
	"github.com/corona10/goimagehash"
)

type MemeHash struct {
	MemeId int
	Hash   *goimagehash.ImageHash
}

func getImageHash(url string) (string, error) {

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("Cannot download image %s. Reason %s", url, err)
	}
	defer resp.Body.Close()
	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Cannot decode image %s. Reason %s", url, err)
	}
	hash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return "", fmt.Errorf("Cannot calculate hash. Reason %s", err)
	}
	return hash.ToString(), nil
}

func getHash(m *pb.Meme) (*goimagehash.ImageHash, error) {
	res := ""
	for _, url := range m.Pictures {
		hash, err := getImageHash(url)
		if err != nil {
			return nil, fmt.Errorf("Cannot get hash from meme. Reason %s", err)
		}
		res = fmt.Sprintf("%s%s|", res, hash)
	}
	hash, err := goimagehash.ImageHashFromString(res)
	if err != nil {
		return nil, fmt.Errorf("Cannot parse image hash from string (meme %v). Reason %s", m, err)
	}
	return hash, nil
}

func (s *Storage) isUnique(meme *pb.Meme) (bool, string, error) {
	rows, err := s.DB.Query("SELECT meme_id, hash FROM meme_hashes")
	if err != nil {
		return false, "", fmt.Errorf("Cannot select hashes for meme %v. Reason %s", meme, err)
	}
	defer rows.Close()

	hashes := []MemeHash{}

	for rows.Next() {
		var memeid int
		var hash string
		err := rows.Scan(&memeid, &hash)
		if err != nil {
			return false, "", fmt.Errorf("Cannot scan image hash from db. Reason %s", err)
		}
		imgHash, err := goimagehash.ImageHashFromString(hash)
		if err != nil {
			Log.Errorf("Cannot parse hash for meme with id %d. Reason %s", memeid, err)
		}
		hashes = append(hashes, MemeHash{
			MemeId: memeid,
			Hash:   imgHash,
		})
	}

	memeHash, err := getHash(meme)
	if err != nil {
		return false, "", fmt.Errorf("Cannot get hash for meme. Reason %s", err)
	}

	for _, hash := range hashes {
		dist, _ := hash.Hash.Distance(memeHash)

		if dist <= Config.Collision.Distance {
			m, err := s.GetMemeById(hash.MemeId)
			if err != nil {
				return false, "", fmt.Errorf("Cannot get meme associated with hash %v. Reason %s", hash, err)
			}

			if m.Description == meme.Description {
				Log.Infof("Meme %v is not unique. Same meme is %v. Hashes %s %s", meme, m, memeHash, hash.Hash)
				return false, memeHash.ToString(), nil
			} else {
				Log.Infof("Pictures in meme %v is not unique to %v, but text is different", meme, m)
			}
		}
	}
	return true, memeHash.ToString(), nil
}
