package application

import (
	"crypto/rand"
	"math/big"
)

// Alfabeto sin caracteres que se confunden al dictarlos (0/O, 1/l/I).
const (
	tempUpper  = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	tempLower  = "abcdefghijkmnpqrstuvwxyz"
	tempDigits = "23456789"
	tempLen    = 12
)

// TemporaryPassword genera una contraseña temporal aleatoria que cumple la
// política del realm (8+ caracteres, mayúscula, número) y se puede dictar.
func TemporaryPassword() (string, error) {
	all := tempUpper + tempLower + tempDigits
	out := make([]byte, tempLen)
	// Garantiza al menos una mayúscula, una minúscula y un número.
	for i, set := range []string{tempUpper, tempLower, tempDigits} {
		c, err := pick(set)
		if err != nil {
			return "", err
		}
		out[i] = c
	}
	for i := 3; i < tempLen; i++ {
		c, err := pick(all)
		if err != nil {
			return "", err
		}
		out[i] = c
	}
	// Mezcla (Fisher-Yates) para que las obligatorias no queden al inicio.
	for i := tempLen - 1; i > 0; i-- {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", err
		}
		out[i], out[j.Int64()] = out[j.Int64()], out[i]
	}
	return string(out), nil
}

func pick(set string) (byte, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(set))))
	if err != nil {
		return 0, err
	}
	return set[n.Int64()], nil
}
