package ftp

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net"
	"unsafe"
	"time"
	"io"
	"strings"
	"net/url"
	"strconv"
)

const (
	maxFileSize = 90 * 1024 * 1024 // 90MB límite
	timeout     = 30 * time.Second
)

func parsePASV(resp string) (string, error) {
	start := strings.Index(resp, "(")
	end := strings.Index(resp, ")")
	if start == -1 || end == -1 {
		return "", fmt.Errorf("formato PASV inválido")
	}
	parts := strings.Split(resp[start+1:end], ",")
	if len(parts) < 6 {
		return "", fmt.Errorf("formato PASV inválido")
	}
	ip := strings.Join(parts[0:4], ".")
	p1, err1 := strconv.Atoi(parts[4])
	p2, err2 := strconv.Atoi(parts[5])
	if err1 != nil || err2 != nil {
		return "", fmt.Errorf("puerto PASV inválido")
	}
	port := p1*256 + p2
	return fmt.Sprintf("%s:%d", ip, port), nil
}

func GetFTPFile(ftpUrl string) string {
	if ftpUrl == "" {
		return ""
	}
	u, err := url.Parse(ftpUrl)
	if err != nil || u.Scheme != "ftp" {
		return ""
	}
	user := u.User.Username()
	pass, _ := u.User.Password()
	host := u.Host
	path := u.Path
	if !strings.Contains(host, ":") {
		host += ":21"
	}
	if host == "" || user == "" {
		return ""
	}
	conn, err := net.DialTimeout("tcp", host, timeout)
	if err != nil {
		return ""
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(timeout))
	var buf [1024]byte
	n, err := conn.Read(buf[:])
	if err != nil {
		return ""
	}
	if _, err := fmt.Fprintf(conn, "USER %s\r\n", user); err != nil {
		return ""
	}
	n, err = conn.Read(buf[:])
	if err != nil || !strings.HasPrefix(string(buf[:n]), "331") {
		return ""
	}
	if _, err := fmt.Fprintf(conn, "PASS %s\r\n", pass); err != nil {
		return ""
	}
	n, err = conn.Read(buf[:])
	if err != nil || !strings.HasPrefix(string(buf[:n]), "230") {
		return ""
	}
	if _, err := fmt.Fprintf(conn, "TYPE I\r\n"); err != nil {
		return ""
	}
	conn.Read(buf[:])
	if _, err := fmt.Fprintf(conn, "PASV\r\n"); err != nil {
		return ""
	}
	n, err = conn.Read(buf[:])
	if err != nil {
		return ""
	}
	pasvResp := string(buf[:n])
	dataAddr, err := parsePASV(pasvResp)
	if err != nil {
		return ""
	}
	dataConn, err := net.DialTimeout("tcp", dataAddr, timeout)
	if err != nil {
		return ""
	}
	defer dataConn.Close()
	dataConn.SetDeadline(time.Now().Add(timeout))
	if _, err := fmt.Fprintf(conn, "RETR %s\r\n", path); err != nil {
		return ""
	}
	n, err = conn.Read(buf[:])
	if err != nil || !strings.HasPrefix(string(buf[:n]), "150") {
		return ""
	}
	limitedReader := &io.LimitedReader{R: dataConn, N: maxFileSize}
	var buffer bytes.Buffer
	if _, err := io.Copy(&buffer, limitedReader); err != nil {
		return ""
	}
	if limitedReader.N <= 0 {
		return ""
	}
	if buffer.Len() == 0 {
		return ""
	}
	encoded := base64.StdEncoding.EncodeToString(buffer.Bytes())
	return encoded
}

func GetFTPText(ftpUrl string) string {
	if ftpUrl == "" {
		return ""
	}
	u, err := url.Parse(ftpUrl)
	if err != nil || u.Scheme != "ftp" {
		return ""
	}
	user := u.User.Username()
	pass, _ := u.User.Password()
	host := u.Host
	path := u.Path
	if !strings.Contains(host, ":") {
		host += ":21"
	}
	if host == "" || user == "" {
		return ""
	}
	conn, err := net.DialTimeout("tcp", host, timeout)
	if err != nil {
		return ""
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(timeout))
	var buf [1024]byte
	n, err := conn.Read(buf[:])
	if err != nil {
		return ""
	}
	if _, err := fmt.Fprintf(conn, "USER %s\r\n", user); err != nil {
		return ""
	}
	n, err = conn.Read(buf[:])
	if err != nil || !strings.HasPrefix(string(buf[:n]), "331") {
		return ""
	}
	if _, err := fmt.Fprintf(conn, "PASS %s\r\n", pass); err != nil {
		return ""
	}
	n, err = conn.Read(buf[:])
	if err != nil || !strings.HasPrefix(string(buf[:n]), "230") {
		return ""
	}
	if _, err := fmt.Fprintf(conn, "TYPE A\r\n"); err != nil {
		return ""
	}
	conn.Read(buf[:])
	if _, err := fmt.Fprintf(conn, "PASV\r\n"); err != nil {
		return ""
	}
	n, err = conn.Read(buf[:])
	if err != nil {
		return ""
	}
	pasvResp := string(buf[:n])
	dataAddr, err := parsePASV(pasvResp)
	if err != nil {
		return ""
	}
	dataConn, err := net.DialTimeout("tcp", dataAddr, timeout)
	if err != nil {
		return ""
	}
	defer dataConn.Close()
	dataConn.SetDeadline(time.Now().Add(timeout))
	if _, err := fmt.Fprintf(conn, "RETR %s\r\n", path); err != nil {
		return ""
	}
	n, err = conn.Read(buf[:])
	if err != nil || !strings.HasPrefix(string(buf[:n]), "150") {
		return ""
	}
	limitedReader := &io.LimitedReader{R: dataConn, N: maxFileSize}
	var buffer bytes.Buffer

	if _, err := io.Copy(&buffer, limitedReader); err != nil {
		return ""
	}
	if limitedReader.N <= 0 {
		return ""
	}
	if buffer.Len() == 0 {
		return ""
	}
	text := buffer.String()
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSpace(text)
	return text
}

func PutFTPFile(base64Data, ftpUrl string) error {
    if base64Data == "" {
        return -1 // Error: Datos vacíos
    }
    if ftpUrl == "" {
        return -2 // Error: URL vacía
    }
    data, err := base64.StdEncoding.DecodeString(base64Data)
    if err != nil {
        return err // Error decodificando base64
    }
    u, err := url.Parse(ftpUrl)
    if err != nil {
        return err // Error analizando URL
    }
    if u.Scheme != "ftp" {
        return -5 // Error: URL no es FTP
    }
    user := u.User.Username()
    pass, _ := u.User.Password()
    host := u.Host
    path := u.Path
    if !strings.Contains(host, ":") {
        host += ":21"
    }
    if host == "" || user == "" {
        return -6 // Error: Falta host o usuario
    }
    conn, err := net.DialTimeout("tcp", host, timeout)
    if err != nil {
        return -7 // Error de conexión
    }
    defer conn.Close()
    conn.SetDeadline(time.Now().Add(timeout))
    var buf [1024]byte
    n, err := conn.Read(buf[:])
    if err != nil {
        return -8 // Error lectura inicial
    }
    if _, err = fmt.Fprintf(conn, "USER %s\r\n", user); err != nil {
        return -9 // Error enviando usuario
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "331") {
        return -10 // Error autenticación usuario
    }
    if _, err = fmt.Fprintf(conn, "PASS %s\r\n", pass); err != nil {
        return -11 // Error enviando contraseña
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "230") {
        return -12 // Error autenticación contraseña
    }
    if _, err = fmt.Fprintf(conn, "TYPE I\r\n"); err != nil {
        return -13 // Error configurando modo binario
    }
    conn.Read(buf[:])
    if _, err = fmt.Fprintf(conn, "PASV\r\n"); err != nil {
        return -14 // Error entrando en modo pasivo
    }
    n, err = conn.Read(buf[:])
    if err != nil {
        return -15 // Error leyendo respuesta PASV
    }
    pasvResp := string(buf[:n])
    dataAddr, err := parsePASV(pasvResp)
    if err != nil {
        return -16 // Error analizando modo pasivo
    }
    dataConn, err := net.DialTimeout("tcp", dataAddr, timeout)
    if err != nil {
        return -17 // Error conexión datos
    }
    defer dataConn.Close()
    dataConn.SetDeadline(time.Now().Add(timeout))
    if _, err = fmt.Fprintf(conn, "STOR %s\r\n", path); err != nil {
        return -18 // Error iniciando transferencia
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "150") {
        return -19 // Error preparando servidor
    }
    _, err = io.Copy(dataConn, bytes.NewReader(data))
    if err != nil {
        return -20 // Error enviando datos
    }
    dataConn.Close()
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "226") {
        return -21 // Error confirmando transferencia
    }
    return nil // Éxito
}

func PutFTPText(textData, ftpUrl string) error {
    if textData == "" {
        return -1 // Error: Texto vacío
    }
    if ftpUrl == "" {
        return -2 // Error: URL vacía
    }
    u, err := url.Parse(ftpUrl)
    if err != nil {
        return err // Error analizando URL
    }
    if u.Scheme != "ftp" {
        return err // Error: URL no es FTP
    }
    user := u.User.Username()
    pass, _ := u.User.Password()
    host := u.Host
    path := u.Path
    if !strings.Contains(host, ":") {
        host += ":21"
    }
    if host == "" || user == "" {
        return -6 // Error: Falta host o usuario
    }
    conn, err := net.DialTimeout("tcp", host, timeout)
    if err != nil {
        return -7 // Error de conexión
    }
    defer conn.Close()
    conn.SetDeadline(time.Now().Add(timeout))
    var buf [1024]byte
    n, err := conn.Read(buf[:])
    if err != nil {
        return -8 // Error lectura inicial
    }
    if _, err = fmt.Fprintf(conn, "USER %s\r\n", user); err != nil {
        return -9 // Error enviando usuario
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "331") {
        return -10 // Error autenticación usuario
    }
    if _, err = fmt.Fprintf(conn, "PASS %s\r\n", pass); err != nil {
        return -11 // Error enviando contraseña
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "230") {
        return -12 // Error autenticación contraseña
    }
    if _, err = fmt.Fprintf(conn, "TYPE A\r\n"); err != nil {
        return -22 // Error configurando modo ASCII
    }
    conn.Read(buf[:])
    if _, err = fmt.Fprintf(conn, "PASV\r\n"); err != nil {
        return -14 // Error entrando en modo pasivo
    }
    n, err = conn.Read(buf[:])
    if err != nil {
        return -15 // Error leyendo respuesta PASV
    }
    pasvResp := string(buf[:n])
    dataAddr, err := parsePASV(pasvResp)
    if err != nil {
        return -16 // Error analizando modo pasivo
    }
    dataConn, err := net.DialTimeout("tcp", dataAddr, timeout)
    if err != nil {
        return -17 // Error conexión datos
    }
    defer dataConn.Close()
    dataConn.SetDeadline(time.Now().Add(timeout))
    if _, err = fmt.Fprintf(conn, "STOR %s\r\n", path); err != nil {
        return -18 // Error iniciando transferencia
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "150") {
        return -19 // Error preparando servidor
    }
    normalizedText := strings.ReplaceAll(textData, "\n", "\r\n")
    _, err = fmt.Fprintf(dataConn, normalizedText)
    if err != nil {
        return -20 // Error enviando datos
    }
    dataConn.Close()
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "226") {
        return -21 // Error confirmando transferencia
    }
    return nil // Éxito
}

func CreateFTPDir(ftpUrl string) error {
    if ftpUrl == "" {
        return -1 // Error: URL vacía
    }
    u, err := url.Parse(ftpUrl)
    if err != nil {
        return err // Error analizando URL
    }
    if u.Scheme != "ftp" {
        return -3 // Error: URL no es FTP
    }
    user := u.User.Username()
    pass, _ := u.User.Password()
    host := u.Host
    path := strings.TrimPrefix(u.Path, "/")
    if !strings.Contains(host, ":") {
        host += ":21"
    }
    if host == "" || user == "" {
        return -4 // Error: Falta host o usuario
    }
    if path == "" {
        return -5 // Error: Falta path del directorio
    }
    conn, err := net.DialTimeout("tcp", host, timeout)
    if err != nil {
        return -6 // Error de conexión
    }
    defer conn.Close()
    conn.SetDeadline(time.Now().Add(timeout))
    var buf [1024]byte
    n, err := conn.Read(buf[:])
    if err != nil {
        return -7 // Error lectura inicial
    }
    if _, err = fmt.Fprintf(conn, "USER %s\r\n", user); err != nil {
        return -8 // Error enviando usuario
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "331") {
        return -9 // Error autenticación usuario
    }
    if _, err = fmt.Fprintf(conn, "PASS %s\r\n", pass); err != nil {
        return -10 // Error enviando contraseña
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "230") {
        return -11 // Error autenticación contraseña
    }
    if _, err = fmt.Fprintf(conn, "SIZE %s\r\n", path); err != nil {
        return -15 // Error enviando comando SIZE
    }
    n, err = conn.Read(buf[:])
    if err != nil {
        return -16 // Error leyendo respuesta SIZE
    }
    sizeResp := string(buf[:n])
    if strings.HasPrefix(sizeResp, "213") {
        return -17 // Ya existe como archivo (conflicto)
    }
    if _, err = fmt.Fprintf(conn, "CWD %s\r\n", path); err != nil {
        return -18 // Error enviando comando CWD
    }
    n, err = conn.Read(buf[:])
    if err != nil {
        return -19 // Error leyendo respuesta CWD
    }
    cwdResp := string(buf[:n])
    if strings.HasPrefix(cwdResp, "250") {
        // Volver al directorio anterior
        _, _ = fmt.Fprintf(conn, "CDUP\r\n")
        return 1 // Ya existe como directorio (éxito relativo)
    }
    if _, err = fmt.Fprintf(conn, "MKD %s\r\n", path); err != nil {
        return -12 // Error enviando comando MKD
    }
    n, err = conn.Read(buf[:])
    if err != nil {
        return -13 // Error leyendo respuesta
    }
    resp := string(buf[:n])
    if !strings.HasPrefix(resp, "257") {
        return -14 // Error creando directorio
    }
    return nil // Éxito (directorio creado)
}

func ListFTPFiles(dirPath string) []string {
    if dirPath == "" {
        return nil
    }
    u, err := url.Parse(dirPath)
    if err != nil || u.Scheme != "ftp" {
        return nil
    }
    user := u.User.Username()
    pass, _ := u.User.Password()
    host := u.Host
    path := u.Path
    if !strings.Contains(host, ":") {
        host += ":21"
    }
    if host == "" || user == "" {
        return nil
    }
    conn, err := net.DialTimeout("tcp", host, timeout)
    if err != nil {
        return nil
    }
    defer conn.Close()
    conn.SetDeadline(time.Now().Add(timeout))
    var buf [1024]byte
    n, err := conn.Read(buf[:])
    if err != nil {
        return nil
    }
    if _, err := fmt.Fprintf(conn, "USER %s\r\n", user); err != nil {
        return nil
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "331") {
        return nil
    }
    if _, err := fmt.Fprintf(conn, "PASS %s\r\n", pass); err != nil {
        return nil
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "230") {
        return nil
    }
    if _, err := fmt.Fprintf(conn, "TYPE A\r\n"); err != nil {
        return nil
    }
    conn.Read(buf[:])
    if _, err := fmt.Fprintf(conn, "PASV\r\n"); err != nil {
        return nil
    }
    n, err = conn.Read(buf[:])
    if err != nil {
        return nil
    }
    pasvResp := string(buf[:n])
    dataAddr, err := parsePASV(pasvResp)
    if err != nil {
        return nil
    }
    dataConn, err := net.DialTimeout("tcp", dataAddr, timeout)
    if err != nil {
        return nil
    }
    defer dataConn.Close()
    dataConn.SetDeadline(time.Now().Add(timeout))
    if _, err := fmt.Fprintf(conn, "LIST %s\r\n", path); err != nil {
        return nil
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "150") {
        return nil
    }
    var buffer bytes.Buffer
    limitedReader := &io.LimitedReader{R: dataConn, N: maxFileSize}
    if _, err := io.Copy(&buffer, limitedReader); err != nil {
        return nil
    }
    dataConn.Close()
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "226") {
        return nil
    }
    lines := strings.Split(buffer.String(), "\n")
    var files []string
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" {
            continue
        }
        parts := strings.Fields(line)
        if len(parts) > 0 {
            files = append(files, parts[len(parts)-1])
        }
    }

    if len(files) == 0 {
        return nil
    }
    return file
}
