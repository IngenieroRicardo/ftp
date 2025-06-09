# FTP
Una biblioteca ligera para hacer peticiones ftp en C  
Compilada usando: `go build -o ftp.dll -buildmode=c-shared ftp.go`

---

### 📥 Descargar la librería

| Linux | Windows |
| --- | --- |
| `wget https://github.com/IngenieroRicardo/ftp/releases/download/1.0/ftp.so` | `Invoke-WebRequest https://github.com/IngenieroRicardo/ftp/releases/download/1.0/ftp.dll -Outftp ./ftp.dll` |
| `wget https://github.com/IngenieroRicardo/ftp/releases/download/1.0/ftp.h` | `Invoke-WebRequest https://github.com/IngenieroRicardo/ftp/releases/download/1.0/ftp.h -Outftp ./ftp.h` |

---

### 🛠️ Compilar

| Linux | Windows |
| --- | --- |
| `gcc -o main.bin main.c ./ftp.so` | `gcc -o main.exe main.c ./ftp.dll` |
| `x86_64-w64-mingw32-gcc -o main.exe main.c ./ftp.dll` |  |

---

### 🧪 Ejemplo de escritura y lectura

```c
#include <stdio.h>
#include <stdlib.h>
#include "ftp.h"

int main() {
    // 1. Ejemplo de escritura binaria desde base64
    char* base64Data = "SGVsbG8gV29ybGQh"; // "Hello World!" en base64
    char* binaryPath = "./salida.bin";

    if (WBftp(base64Data, binaryPath) == 0) {
        printf("Archivo binario creado: %s\n", binaryPath);
    }

    // 2. Ejemplo de escritura de texto
    char* textData = "Este es un texto de ejemplo\nSegunda línea";
    char* textPath = "./salida.txt";

    if (WTftp(textData, textPath) == 0) {
        printf("Archivo de texto creado: %s\n", textPath);
    }

    // 3. Ejemplo de lectura binaria (a base64)
    char* base64Result = RBftp(binaryPath);
    if (base64Result != NULL) {
        printf("Base64 del archivo binario: %s\n", base64Result);
        free(base64Result);
    }

    // 4. Ejemplo de lectura de texto
    char* textResult = RTftp(textPath);
    if (textResult != NULL) {
        printf("Contenido del archivo de texto:\n%s\n", textResult);
        free(textResult);
    }

    return 0;
}
```

---

### 🧪 Ejemplo de obtención de content-type

```c
#include <stdio.h>
#include <stdlib.h>
#include "ftp.h"

int main() {
    // Ejemplo con GetContentTypeFromBase64
    char* imageBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z/C/HgAGgwJ/lK3Q6wAAAABJRU5ErkJggg=="; // PNG 1x1
    char* contentType = GetContentTypeftp(imageBase64);
    
    printf("Content-Type: %s\n", contentType);
    free(contentType);
    
    // Ejemplo con JSON
    char* jsonBase64 = "ewogICJuYW1lIjogIkpvaG4gRG9lIiwKICAiYWdlIjogMzAKfQ=="; // {"name": "John Doe", "age": 30}
    contentType = GetContentTypeftp(jsonBase64);
    
    printf("Content-Type: %s\n", contentType);
    free(contentType);
    
    return 0;
}
```

---

### 🧪 Ejemplo de directorio

```c
#include <stdio.h>
#include <stdlib.h>
#include "ftp.h"

int main() {
    char* dirPath = "."; // Directorio actual

    // Obtener lista de archivos
    char** ftps = Listftps(dirPath);

    if (ftps != NULL) {
        printf("Archivos en el directorio '%s':\n", dirPath);

        // Iterar hasta encontrar el terminador NULL
        for (int i = 0; ftps[i] != NULL; i++) {
            printf("- %s\n", ftps[i]);
        }

        // Liberar memoria
        FreeListftps(ftps);
    } else {
        printf("Error al leer el directorio o directorio vacío\n");
    }

    return 0;
}
```

---

## 📚 Documentación de la API

#### Manejo de archivos binarios
- `char* GetFTPFile(char* ftpUrl)`: Retorna el Base64 del archivo leído.
- `int PutFTPFile(char* b64Str, char* ftpUrl)`: Retorna 0 cuando el archivo se crea correctamente.

#### Manejo de archivos de texto
- `char* GetFTPText(char* ftpUrl)`: Retorna el texto del archivo leído.
- `int PutFTPText(char* b64Str, char* ftpUrl)`: Retorna 0 cuando el archivo se crea correctamente.

#### Manejo de directorios
- `int CreateFTPDir(char* ftpUrl)`: Retorna 0 cuando el directorio se crea correctamente.
- `char** ListFTPFiles(char* ftpUrl)`: Retorna la lista de archivos en la ruta.

#### Utilidades
- `void FreeFTPList(char** ftps)`: Libera la memoria de resultados.
