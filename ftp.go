package main

/*
#include <stdlib.h>
*/
import "C"
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
	maxFileSize = 90 * 1024 * 1024 // 10MB límite
	timeout     = 30 * time.Second
)

//export GetFTPFile
func GetFTPFile(ftpUrl *C.char) *C.char {
	urlStr := C.GoString(ftpUrl)

	if urlStr == "" {
		return nil
	}

	u, err := url.Parse(urlStr)
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

	if _, err := fmt.Fprintf(conn, "TYPE I\r\n"); err != nil {
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

	if _, err := fmt.Fprintf(conn, "RETR %s\r\n", path); err != nil {
		return nil
	}
	n, err = conn.Read(buf[:])
	if err != nil || !strings.HasPrefix(string(buf[:n]), "150") {
		return nil
	}

	limitedReader := &io.LimitedReader{R: dataConn, N: maxFileSize}
	var buffer bytes.Buffer

	if _, err := io.Copy(&buffer, limitedReader); err != nil {
		return nil
	}

	if limitedReader.N <= 0 {
		return nil
	}

	if buffer.Len() == 0 {
		return nil
	}

	encoded := base64.StdEncoding.EncodeToString(buffer.Bytes())
	return C.CString(encoded)
}

//export GetFTPText
func GetFTPText(ftpUrl *C.char) *C.char {
	urlStr := C.GoString(ftpUrl)

	if urlStr == "" {
		return nil
	}

	u, err := url.Parse(urlStr)
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

	if _, err := fmt.Fprintf(conn, "RETR %s\r\n", path); err != nil {
		return nil
	}
	n, err = conn.Read(buf[:])
	if err != nil || !strings.HasPrefix(string(buf[:n]), "150") {
		return nil
	}

	limitedReader := &io.LimitedReader{R: dataConn, N: maxFileSize}
	var buffer bytes.Buffer

	if _, err := io.Copy(&buffer, limitedReader); err != nil {
		return nil
	}

	if limitedReader.N <= 0 {
		return nil
	}

	if buffer.Len() == 0 {
		return nil
	}

	text := buffer.String()
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSpace(text)

	return C.CString(text)
}

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

//export PutFTPFile
func PutFTPFile(base64Data, ftpUrl *C.char) C.int {
    // Convertir C strings a Go strings
    base64Str := C.GoString(base64Data)
    urlStr := C.GoString(ftpUrl)

    // Validar entrada
    if base64Str == "" {
        return C.int(-1) // Error: Datos vacíos
    }

    if urlStr == "" {
        return C.int(-2) // Error: URL vacía
    }

    // Decodificar base64
    data, err := base64.StdEncoding.DecodeString(base64Str)
    if err != nil {
        return C.int(-3) // Error decodificando base64
    }

    // Parsear URL FTP
    u, err := url.Parse(urlStr)
    if err != nil {
        return C.int(-4) // Error analizando URL
    }

    if u.Scheme != "ftp" {
        return C.int(-5) // Error: URL no es FTP
    }

    // Extraer credenciales y ruta
    user := u.User.Username()
    pass, _ := u.User.Password()
    host := u.Host
    path := u.Path

    if !strings.Contains(host, ":") {
        host += ":21"
    }

    if host == "" || user == "" {
        return C.int(-6) // Error: Falta host o usuario
    }

    // Establecer conexión FTP
    conn, err := net.DialTimeout("tcp", host, timeout)
    if err != nil {
        return C.int(-7) // Error de conexión
    }
    defer conn.Close()
    conn.SetDeadline(time.Now().Add(timeout))

    var buf [1024]byte
    n, err := conn.Read(buf[:])
    if err != nil {
        return C.int(-8) // Error lectura inicial
    }

    // Autenticación
    if _, err = fmt.Fprintf(conn, "USER %s\r\n", user); err != nil {
        return C.int(-9) // Error enviando usuario
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "331") {
        return C.int(-10) // Error autenticación usuario
    }

    if _, err = fmt.Fprintf(conn, "PASS %s\r\n", pass); err != nil {
        return C.int(-11) // Error enviando contraseña
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "230") {
        return C.int(-12) // Error autenticación contraseña
    }

    // Configurar modo binario
    if _, err = fmt.Fprintf(conn, "TYPE I\r\n"); err != nil {
        return C.int(-13) // Error configurando modo binario
    }
    conn.Read(buf[:])

    // Modo pasivo para transferencia
    if _, err = fmt.Fprintf(conn, "PASV\r\n"); err != nil {
        return C.int(-14) // Error entrando en modo pasivo
    }
    n, err = conn.Read(buf[:])
    if err != nil {
        return C.int(-15) // Error leyendo respuesta PASV
    }

    pasvResp := string(buf[:n])
    dataAddr, err := parsePASV(pasvResp)
    if err != nil {
        return C.int(-16) // Error analizando modo pasivo
    }

    dataConn, err := net.DialTimeout("tcp", dataAddr, timeout)
    if err != nil {
        return C.int(-17) // Error conexión datos
    }
    defer dataConn.Close()
    dataConn.SetDeadline(time.Now().Add(timeout))

    // Comando STOR para subir archivo
    if _, err = fmt.Fprintf(conn, "STOR %s\r\n", path); err != nil {
        return C.int(-18) // Error iniciando transferencia
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "150") {
        return C.int(-19) // Error preparando servidor
    }

    // Enviar datos
    _, err = io.Copy(dataConn, bytes.NewReader(data))
    if err != nil {
        return C.int(-20) // Error enviando datos
    }

    // Cerrar conexión de datos primero
    dataConn.Close()

    // Verificar confirmación de transferencia
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "226") {
        return C.int(-21) // Error confirmando transferencia
    }

    return C.int(0) // Éxito
}

//export PutFTPText
func PutFTPText(textData, ftpUrl *C.char) C.int {
    // Convertir C strings a Go strings
    textStr := C.GoString(textData)
    urlStr := C.GoString(ftpUrl)

    // Validar entrada
    if textStr == "" {
        return C.int(-1) // Error: Texto vacío
    }

    if urlStr == "" {
        return C.int(-2) // Error: URL vacía
    }

    // Parsear URL FTP
    u, err := url.Parse(urlStr)
    if err != nil {
        return C.int(-4) // Error analizando URL
    }

    if u.Scheme != "ftp" {
        return C.int(-5) // Error: URL no es FTP
    }

    // Extraer credenciales y ruta
    user := u.User.Username()
    pass, _ := u.User.Password()
    host := u.Host
    path := u.Path

    if !strings.Contains(host, ":") {
        host += ":21"
    }

    if host == "" || user == "" {
        return C.int(-6) // Error: Falta host o usuario
    }

    // Establecer conexión FTP
    conn, err := net.DialTimeout("tcp", host, timeout)
    if err != nil {
        return C.int(-7) // Error de conexión
    }
    defer conn.Close()
    conn.SetDeadline(time.Now().Add(timeout))

    var buf [1024]byte
    n, err := conn.Read(buf[:])
    if err != nil {
        return C.int(-8) // Error lectura inicial
    }

    // Autenticación
    if _, err = fmt.Fprintf(conn, "USER %s\r\n", user); err != nil {
        return C.int(-9) // Error enviando usuario
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "331") {
        return C.int(-10) // Error autenticación usuario
    }

    if _, err = fmt.Fprintf(conn, "PASS %s\r\n", pass); err != nil {
        return C.int(-11) // Error enviando contraseña
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "230") {
        return C.int(-12) // Error autenticación contraseña
    }

    // Configurar modo ASCII para texto
    if _, err = fmt.Fprintf(conn, "TYPE A\r\n"); err != nil {
        return C.int(-22) // Error configurando modo ASCII
    }
    conn.Read(buf[:])

    // Modo pasivo para transferencia
    if _, err = fmt.Fprintf(conn, "PASV\r\n"); err != nil {
        return C.int(-14) // Error entrando en modo pasivo
    }
    n, err = conn.Read(buf[:])
    if err != nil {
        return C.int(-15) // Error leyendo respuesta PASV
    }

    pasvResp := string(buf[:n])
    dataAddr, err := parsePASV(pasvResp)
    if err != nil {
        return C.int(-16) // Error analizando modo pasivo
    }

    dataConn, err := net.DialTimeout("tcp", dataAddr, timeout)
    if err != nil {
        return C.int(-17) // Error conexión datos
    }
    defer dataConn.Close()
    dataConn.SetDeadline(time.Now().Add(timeout))

    // Comando STOR para subir archivo
    if _, err = fmt.Fprintf(conn, "STOR %s\r\n", path); err != nil {
        return C.int(-18) // Error iniciando transferencia
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "150") {
        return C.int(-19) // Error preparando servidor
    }

    // Normalizar saltos de línea a CRLF (estándar FTP para texto)
    normalizedText := strings.ReplaceAll(textStr, "\n", "\r\n")
    
    // Enviar datos
    _, err = fmt.Fprintf(dataConn, normalizedText)
    if err != nil {
        return C.int(-20) // Error enviando datos
    }

    // Cerrar conexión de datos primero
    dataConn.Close()

    // Verificar confirmación de transferencia
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "226") {
        return C.int(-21) // Error confirmando transferencia
    }

    return C.int(0) // Éxito
}


//export CreateFTPDir
func CreateFTPDir(ftpUrl *C.char) C.int {
    urlStr := C.GoString(ftpUrl)

    // Validación básica
    if urlStr == "" {
        return C.int(-1) // Error: URL vacía
    }

    // Parsear URL FTP
    u, err := url.Parse(urlStr)
    if err != nil {
        return C.int(-2) // Error analizando URL
    }

    if u.Scheme != "ftp" {
        return C.int(-3) // Error: URL no es FTP
    }

    // Extraer credenciales y ruta
    user := u.User.Username()
    pass, _ := u.User.Password()
    host := u.Host
    path := strings.TrimPrefix(u.Path, "/")

    if !strings.Contains(host, ":") {
        host += ":21"
    }

    if host == "" || user == "" {
        return C.int(-4) // Error: Falta host o usuario
    }

    if path == "" {
        return C.int(-5) // Error: Falta path del directorio
    }

    // Establecer conexión FTP
    conn, err := net.DialTimeout("tcp", host, timeout)
    if err != nil {
        return C.int(-6) // Error de conexión
    }
    defer conn.Close()
    conn.SetDeadline(time.Now().Add(timeout))

    var buf [1024]byte
    n, err := conn.Read(buf[:])
    if err != nil {
        return C.int(-7) // Error lectura inicial
    }

    // Autenticación
    if _, err = fmt.Fprintf(conn, "USER %s\r\n", user); err != nil {
        return C.int(-8) // Error enviando usuario
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "331") {
        return C.int(-9) // Error autenticación usuario
    }

    if _, err = fmt.Fprintf(conn, "PASS %s\r\n", pass); err != nil {
        return C.int(-10) // Error enviando contraseña
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "230") {
        return C.int(-11) // Error autenticación contraseña
    }

    // PRIMERO: Verificar si ya existe como archivo (comando SIZE)
    if _, err = fmt.Fprintf(conn, "SIZE %s\r\n", path); err != nil {
        return C.int(-15) // Error enviando comando SIZE
    }
    n, err = conn.Read(buf[:])
    if err != nil {
        return C.int(-16) // Error leyendo respuesta SIZE
    }
    
    sizeResp := string(buf[:n])
    if strings.HasPrefix(sizeResp, "213") {
        return C.int(-17) // Ya existe como archivo (conflicto)
    }

    // SEGUNDO: Verificar si ya existe como directorio (comando CWD)
    if _, err = fmt.Fprintf(conn, "CWD %s\r\n", path); err != nil {
        return C.int(-18) // Error enviando comando CWD
    }
    n, err = conn.Read(buf[:])
    if err != nil {
        return C.int(-19) // Error leyendo respuesta CWD
    }
    
    cwdResp := string(buf[:n])
    if strings.HasPrefix(cwdResp, "250") {
        // Volver al directorio anterior
        _, _ = fmt.Fprintf(conn, "CDUP\r\n")
        return C.int(1) // Ya existe como directorio (éxito relativo)
    }

    // TERCERO: Crear el directorio (comando MKD)
    if _, err = fmt.Fprintf(conn, "MKD %s\r\n", path); err != nil {
        return C.int(-12) // Error enviando comando MKD
    }

    // Leer respuesta
    n, err = conn.Read(buf[:])
    if err != nil {
        return C.int(-13) // Error leyendo respuesta
    }

    resp := string(buf[:n])
    if !strings.HasPrefix(resp, "257") {
        return C.int(-14) // Error creando directorio
    }

    return C.int(0) // Éxito (directorio creado)
}


//export ListFTPFiles
func ListFTPFiles(dirPath *C.char) **C.char {
    urlStr := C.GoString(dirPath)

    // Validación básica
    if urlStr == "" {
        return nil
    }

    // Parsear URL FTP
    u, err := url.Parse(urlStr)
    if err != nil || u.Scheme != "ftp" {
        return nil
    }

    // Extraer credenciales y ruta
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

    // Establecer conexión FTP
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

    // Autenticación
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

    // Configurar modo ASCII para listado
    if _, err := fmt.Fprintf(conn, "TYPE A\r\n"); err != nil {
        return nil
    }
    conn.Read(buf[:])

    // Modo pasivo para transferencia
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

    // Comando LIST para obtener el listado
    if _, err := fmt.Fprintf(conn, "LIST %s\r\n", path); err != nil {
        return nil
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "150") {
        return nil
    }

    // Leer el listado de archivos
    var buffer bytes.Buffer
    limitedReader := &io.LimitedReader{R: dataConn, N: maxFileSize}
    if _, err := io.Copy(&buffer, limitedReader); err != nil {
        return nil
    }

    // Cerrar conexión de datos primero
    dataConn.Close()

    // Verificar confirmación de transferencia
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "226") {
        return nil
    }

    // Procesar el listado de archivos
    lines := strings.Split(buffer.String(), "\n")
    var files []string
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" {
            continue
        }
        // Extraer el nombre del archivo (última parte de la línea)
        parts := strings.Fields(line)
        if len(parts) > 0 {
            files = append(files, parts[len(parts)-1])
        }
    }

    if len(files) == 0 {
        return nil
    }

    // Crear array de C strings
    cArray := C.malloc(C.size_t(len(files)) * C.size_t(unsafe.Sizeof(uintptr(0))))
    if cArray == nil {
        return nil
    }

    // Convertir el array C a slice Go
    goArray := (*[1<<30 - 1]*C.char)(unsafe.Pointer(cArray))[:len(files):len(files)]

    for i, file := range files {
        goArray[i] = C.CString(file)
    }

    return (**C.char)(cArray)
}

//export FreeFTPList
func FreeFTPList(arr **C.char) {
	if arr == nil {
		return
	}
	for i := 0; ; i++ {
		p := *(**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(arr)) + uintptr(i)*unsafe.Sizeof(*arr)))
		if p == nil {
			break
		}
		C.free(unsafe.Pointer(p))
	}
	C.free(unsafe.Pointer(arr))
}


func main() {}