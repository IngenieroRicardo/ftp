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
    "path/filepath"
    "strconv"
    "github.com/pkg/sftp"
    "golang.org/x/crypto/ssh"
)

const (
    maxFileSize = 90 * 1024 * 1024 // 90MB límite
    timeout     = 30 * time.Second
)

// Error codes (compatible with C)
const (
    ErrEmptyData        = -1
    ErrEmptyURL         = -2
    ErrInvalidScheme    = -3
    ErrMissingHostUser  = -4
    ErrMissingPath      = -5
    ErrConnectionFailed = -6
    ErrInitialRead      = -7
    ErrUserSend         = -8
    ErrUserAuth         = -9
    ErrPassSend         = -10
    ErrPassAuth         = -11
    ErrMkdirFailed      = -12
    ErrMkdirResponse    = -13
    ErrPasvMode         = -14
    ErrSizeCommand      = -15
    ErrSizeResponse     = -16
    ErrFileConflict     = -17
    ErrCwdCommand       = -18
    ErrCwdResponse      = -19
    ErrTypeCommand      = -20
    ErrStorCommand      = -21
    ErrDataTransfer     = -22
    ErrTransferConfirm  = -23
    ErrAsciiMode        = -24
    ErrSftpConnection   = -25
    ErrSftpClient       = -26
    ErrSftpOperation    = -27
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

func isSFTP(urlStr string) bool {
    u, err := url.Parse(urlStr)
    if err != nil {
        return false
    }
    return u.Scheme == "sftp"
}

func createSFTPClient(ftpUrl string) (*sftp.Client, error) {
    u, err := url.Parse(ftpUrl)
    if err != nil {
        return nil, fmt.Errorf("error parsing URL: %v", err)
    }

    user := u.User.Username()
    pass, _ := u.User.Password()
    host := u.Host

    if !strings.Contains(host, ":") {
        host += ":22"
    }

    config := &ssh.ClientConfig{
        User: user,
        Auth: []ssh.AuthMethod{
            ssh.Password(pass),
        },
        HostKeyCallback: ssh.InsecureIgnoreHostKey(),
        Timeout:         timeout,
    }

    conn, err := ssh.Dial("tcp", host, config)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to SFTP server: %v", err)
    }

    client, err := sftp.NewClient(conn)
    if err != nil {
        return nil, fmt.Errorf("failed to create SFTP client: %v", err)
    }

    return client, nil
}

//export GetFTPFile
func GetFTPFile(ftpUrl *C.char) *C.char {
    urlStr := C.GoString(ftpUrl)
    if urlStr == "" {
        return nil
    }

    if isSFTP(urlStr) {
        client, err := createSFTPClient(urlStr)
        if err != nil {
            return nil
        }
        defer client.Close()

        u, _ := url.Parse(urlStr)
        path := u.Path

        file, err := client.Open(path)
        if err != nil {
            return nil
        }
        defer file.Close()

        limitedReader := &io.LimitedReader{R: file, N: maxFileSize}
        var buffer bytes.Buffer
        if _, err := io.Copy(&buffer, limitedReader); err != nil {
            return nil
        }

        if limitedReader.N <= 0 || buffer.Len() == 0 {
            return nil
        }

        encoded := base64.StdEncoding.EncodeToString(buffer.Bytes())
        return C.CString(encoded)
    }

    // Original FTP implementation
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

    if isSFTP(urlStr) {
        client, err := createSFTPClient(urlStr)
        if err != nil {
            return nil
        }
        defer client.Close()

        u, _ := url.Parse(urlStr)
        path := u.Path

        file, err := client.Open(path)
        if err != nil {
            return nil
        }
        defer file.Close()

        limitedReader := &io.LimitedReader{R: file, N: maxFileSize}
        var buffer bytes.Buffer
        if _, err := io.Copy(&buffer, limitedReader); err != nil {
            return nil
        }

        if limitedReader.N <= 0 || buffer.Len() == 0 {
            return nil
        }

        text := buffer.String()
        text = strings.ReplaceAll(text, "\r\n", "\n")
        text = strings.TrimSpace(text)
        return C.CString(text)
    }

    // Original FTP implementation
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

//export PutFTPFile
func PutFTPFile(base64Data, ftpUrl *C.char) C.int {
    base64Str := C.GoString(base64Data)
    urlStr := C.GoString(ftpUrl)
    if base64Str == "" {
        return C.int(ErrEmptyData)
    }
    if urlStr == "" {
        return C.int(ErrEmptyURL)
    }

    data, err := base64.StdEncoding.DecodeString(base64Str)
    if err != nil {
        return C.int(-3) // Error decodificando base64
    }

    if isSFTP(urlStr) {
        client, err := createSFTPClient(urlStr)
        if err != nil {
            return C.int(ErrSftpConnection)
        }
        defer client.Close()

        u, _ := url.Parse(urlStr)
        path := u.Path

        // Create parent directories if they don't exist
        dir := filepath.Dir(path)
        if dir != "." {
            if err := client.MkdirAll(dir); err != nil {
                return C.int(ErrSftpOperation)
            }
        }

        file, err := client.Create(path)
        if err != nil {
            return C.int(ErrSftpOperation)
        }
        defer file.Close()

        if _, err := file.Write(data); err != nil {
            return C.int(ErrSftpOperation)
        }

        return C.int(0) // Success
    }

    // Original FTP implementation
    u, err := url.Parse(urlStr)
    if err != nil {
        return C.int(ErrEmptyURL)
    }
    if u.Scheme != "ftp" {
        return C.int(ErrInvalidScheme)
    }

    user := u.User.Username()
    pass, _ := u.User.Password()
    host := u.Host
    path := u.Path

    if !strings.Contains(host, ":") {
        host += ":21"
    }

    if host == "" || user == "" {
        return C.int(ErrMissingHostUser)
    }

    conn, err := net.DialTimeout("tcp", host, timeout)
    if err != nil {
        return C.int(ErrConnectionFailed)
    }
    defer conn.Close()
    conn.SetDeadline(time.Now().Add(timeout))

    var buf [1024]byte
    n, err := conn.Read(buf[:])
    if err != nil {
        return C.int(ErrInitialRead)
    }

    if _, err = fmt.Fprintf(conn, "USER %s\r\n", user); err != nil {
        return C.int(ErrUserSend)
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "331") {
        return C.int(ErrUserAuth)
    }

    if _, err = fmt.Fprintf(conn, "PASS %s\r\n", pass); err != nil {
        return C.int(ErrPassSend)
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "230") {
        return C.int(ErrPassAuth)
    }

    if _, err = fmt.Fprintf(conn, "TYPE I\r\n"); err != nil {
        return C.int(ErrTypeCommand)
    }
    conn.Read(buf[:])

    if _, err = fmt.Fprintf(conn, "PASV\r\n"); err != nil {
        return C.int(ErrPasvMode)
    }
    n, err = conn.Read(buf[:])
    if err != nil {
        return C.int(ErrPasvMode)
    }

    pasvResp := string(buf[:n])
    dataAddr, err := parsePASV(pasvResp)
    if err != nil {
        return C.int(ErrPasvMode)
    }

    dataConn, err := net.DialTimeout("tcp", dataAddr, timeout)
    if err != nil {
        return C.int(ErrConnectionFailed)
    }
    defer dataConn.Close()
    dataConn.SetDeadline(time.Now().Add(timeout))

    if _, err = fmt.Fprintf(conn, "STOR %s\r\n", path); err != nil {
        return C.int(ErrStorCommand)
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "150") {
        return C.int(ErrDataTransfer)
    }

    _, err = io.Copy(dataConn, bytes.NewReader(data))
    if err != nil {
        return C.int(ErrDataTransfer)
    }
    dataConn.Close()

    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "226") {
        return C.int(ErrTransferConfirm)
    }

    return C.int(0) // Success
}

//export PutFTPText
func PutFTPText(textData, ftpUrl *C.char) C.int {
    textStr := C.GoString(textData)
    urlStr := C.GoString(ftpUrl)
    if textStr == "" {
        return C.int(ErrEmptyData)
    }
    if urlStr == "" {
        return C.int(ErrEmptyURL)
    }

    if isSFTP(urlStr) {
        client, err := createSFTPClient(urlStr)
        if err != nil {
            return C.int(ErrSftpConnection)
        }
        defer client.Close()

        u, _ := url.Parse(urlStr)
        path := u.Path

        // Create parent directories if they don't exist
        dir := filepath.Dir(path)
        if dir != "." {
            if err := client.MkdirAll(dir); err != nil {
                return C.int(ErrSftpOperation)
            }
        }

        file, err := client.Create(path)
        if err != nil {
            return C.int(ErrSftpOperation)
        }
        defer file.Close()

        normalizedText := strings.ReplaceAll(textStr, "\n", "\r\n")
        if _, err := fmt.Fprintf(file, normalizedText); err != nil {
            return C.int(ErrSftpOperation)
        }

        return C.int(0) // Success
    }

    // Original FTP implementation
    u, err := url.Parse(urlStr)
    if err != nil {
        return C.int(ErrEmptyURL)
    }
    if u.Scheme != "ftp" {
        return C.int(ErrInvalidScheme)
    }

    user := u.User.Username()
    pass, _ := u.User.Password()
    host := u.Host
    path := u.Path

    if !strings.Contains(host, ":") {
        host += ":21"
    }

    if host == "" || user == "" {
        return C.int(ErrMissingHostUser)
    }

    conn, err := net.DialTimeout("tcp", host, timeout)
    if err != nil {
        return C.int(ErrConnectionFailed)
    }
    defer conn.Close()
    conn.SetDeadline(time.Now().Add(timeout))

    var buf [1024]byte
    n, err := conn.Read(buf[:])
    if err != nil {
        return C.int(ErrInitialRead)
    }

    if _, err = fmt.Fprintf(conn, "USER %s\r\n", user); err != nil {
        return C.int(ErrUserSend)
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "331") {
        return C.int(ErrUserAuth)
    }

    if _, err = fmt.Fprintf(conn, "PASS %s\r\n", pass); err != nil {
        return C.int(ErrPassSend)
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "230") {
        return C.int(ErrPassAuth)
    }

    if _, err = fmt.Fprintf(conn, "TYPE A\r\n"); err != nil {
        return C.int(ErrAsciiMode)
    }
    conn.Read(buf[:])

    if _, err = fmt.Fprintf(conn, "PASV\r\n"); err != nil {
        return C.int(ErrPasvMode)
    }
    n, err = conn.Read(buf[:])
    if err != nil {
        return C.int(ErrPasvMode)
    }

    pasvResp := string(buf[:n])
    dataAddr, err := parsePASV(pasvResp)
    if err != nil {
        return C.int(ErrPasvMode)
    }

    dataConn, err := net.DialTimeout("tcp", dataAddr, timeout)
    if err != nil {
        return C.int(ErrConnectionFailed)
    }
    defer dataConn.Close()
    dataConn.SetDeadline(time.Now().Add(timeout))

    if _, err = fmt.Fprintf(conn, "STOR %s\r\n", path); err != nil {
        return C.int(ErrStorCommand)
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "150") {
        return C.int(ErrDataTransfer)
    }

    normalizedText := strings.ReplaceAll(textStr, "\n", "\r\n")
    _, err = fmt.Fprintf(dataConn, normalizedText)
    if err != nil {
        return C.int(ErrDataTransfer)
    }
    dataConn.Close()

    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "226") {
        return C.int(ErrTransferConfirm)
    }

    return C.int(0) // Success
}

//export CreateFTPDir
func CreateFTPDir(ftpUrl *C.char) C.int {
    urlStr := C.GoString(ftpUrl)
    if urlStr == "" {
        return C.int(ErrEmptyURL)
    }

    if isSFTP(urlStr) {
        client, err := createSFTPClient(urlStr)
        if err != nil {
            return C.int(ErrSftpConnection)
        }
        defer client.Close()

        u, _ := url.Parse(urlStr)
        path := strings.TrimPrefix(u.Path, "/")

        if path == "" {
            return C.int(ErrMissingPath)
        }

        // Check if path exists as a file
        if stat, err := client.Stat(path); err == nil {
            if !stat.IsDir() {
                return C.int(ErrFileConflict)
            }
            return C.int(1) // Already exists as directory
        }

        // Create the directory
        if err := client.MkdirAll(path); err != nil {
            return C.int(ErrMkdirFailed)
        }

        return C.int(0) // Success
    }

    // Original FTP implementation
    u, err := url.Parse(urlStr)
    if err != nil {
        return C.int(ErrEmptyURL)
    }
    if u.Scheme != "ftp" {
        return C.int(ErrInvalidScheme)
    }

    user := u.User.Username()
    pass, _ := u.User.Password()
    host := u.Host
    path := strings.TrimPrefix(u.Path, "/")

    if !strings.Contains(host, ":") {
        host += ":21"
    }

    if host == "" || user == "" {
        return C.int(ErrMissingHostUser)
    }

    if path == "" {
        return C.int(ErrMissingPath)
    }

    conn, err := net.DialTimeout("tcp", host, timeout)
    if err != nil {
        return C.int(ErrConnectionFailed)
    }
    defer conn.Close()
    conn.SetDeadline(time.Now().Add(timeout))

    var buf [1024]byte
    n, err := conn.Read(buf[:])
    if err != nil {
        return C.int(ErrInitialRead)
    }

    if _, err = fmt.Fprintf(conn, "USER %s\r\n", user); err != nil {
        return C.int(ErrUserSend)
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "331") {
        return C.int(ErrUserAuth)
    }

    if _, err = fmt.Fprintf(conn, "PASS %s\r\n", pass); err != nil {
        return C.int(ErrPassSend)
    }
    n, err = conn.Read(buf[:])
    if err != nil || !strings.HasPrefix(string(buf[:n]), "230") {
        return C.int(ErrPassAuth)
    }

    if _, err = fmt.Fprintf(conn, "SIZE %s\r\n", path); err != nil {
        return C.int(ErrSizeCommand)
    }
    n, err = conn.Read(buf[:])
    if err != nil {
        return C.int(ErrSizeResponse)
    }
    sizeResp := string(buf[:n])
    if strings.HasPrefix(sizeResp, "213") {
        return C.int(ErrFileConflict)
    }

    if _, err = fmt.Fprintf(conn, "CWD %s\r\n", path); err != nil {
        return C.int(ErrCwdCommand)
    }
    n, err = conn.Read(buf[:])
    if err != nil {
        return C.int(ErrCwdResponse)
    }
    cwdResp := string(buf[:n])
    if strings.HasPrefix(cwdResp, "250") {
        // Volver al directorio anterior
        _, _ = fmt.Fprintf(conn, "CDUP\r\n")
        return C.int(1) // Ya existe como directorio
    }

    if _, err = fmt.Fprintf(conn, "MKD %s\r\n", path); err != nil {
        return C.int(ErrMkdirFailed)
    }
    n, err = conn.Read(buf[:])
    if err != nil {
        return C.int(ErrMkdirResponse)
    }
    resp := string(buf[:n])
    if !strings.HasPrefix(resp, "257") {
        return C.int(ErrMkdirFailed)
    }

    return C.int(0) // Success
}

//export ListFTPFiles
func ListFTPFiles(dirPath *C.char) **C.char {
    urlStr := C.GoString(dirPath)
    if urlStr == "" {
        return nil
    }

    if isSFTP(urlStr) {
        client, err := createSFTPClient(urlStr)
        if err != nil {
            return nil
        }
        defer client.Close()

        u, _ := url.Parse(urlStr)
        path := u.Path

        files, err := client.ReadDir(path)
        if err != nil {
            return nil
        }

        var fileNames []string
        for _, file := range files {
            fileNames = append(fileNames, file.Name())
        }

        if len(fileNames) == 0 {
            return nil
        }

        // Allocate space for the array plus one extra for NULL terminator
        cArray := C.malloc(C.size_t(len(fileNames)+1) * C.size_t(unsafe.Sizeof(uintptr(0))))
        if cArray == nil {
            return nil
        }
        goArray := (*[1<<30 - 1]*C.char)(unsafe.Pointer(cArray))[:len(fileNames)+1:len(fileNames)+1]
        for i, file := range fileNames {
            goArray[i] = C.CString(file)
        }
        goArray[len(fileNames)] = nil // NULL terminator
        return (**C.char)(cArray)
    }

    // Original FTP implementation
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

    // Allocate space for the array plus one extra for NULL terminator
    cArray := C.malloc(C.size_t(len(files)+1) * C.size_t(unsafe.Sizeof(uintptr(0))))
    if cArray == nil {
        return nil
    }
    goArray := (*[1<<30 - 1]*C.char)(unsafe.Pointer(cArray))[:len(files)+1:len(files)+1]
    for i, file := range files {
        goArray[i] = C.CString(file)
    }
    goArray[len(files)] = nil // NULL terminator
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
