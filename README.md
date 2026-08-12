# Regieleki

**Blazing fast PixelDrain TUI downloader**

Regieleki é um downloader de terminal feito em Go, com Cobra, Bubble Tea e Lip Gloss. Ele recebe o ID de um arquivo, monta o endpoint configurado e baixa o arquivo em streaming para o computador.

## Requisitos

- Go 1.26 ou superior
- Conexão com a internet

## Executar em desenvolvimento

Na raiz do projeto:

```powershell
go run .
```

Digite o ID do arquivo na TUI e pressione `Enter`. O download começa automaticamente.

Para escolher a pasta de destino:

```powershell
go run . --output-dir downloads
```

Também é possível usar o caminho curto:

```powershell
go run . -o downloads
```

## Gerar o binário

O script do Windows gera `bin/regieleki.exe` com flags para reduzir o tamanho:

```powershell
.\build.ps1
```

O build usa:

- `-trimpath` para remover caminhos locais do binário;
- `-buildvcs=false` para não incluir metadados do Git;
- `-ldflags="-s -w"` para remover símbolos e informações de debug.

O comando equivalente, em qualquer sistema com Go instalado, é:

```bash
go build -trimpath -buildvcs=false -ldflags="-s -w" -o bin/regieleki .
```

Depois, execute:

```powershell
.\bin\regieleki.exe
```

O executável gerado é ignorado pelo Git; apenas `bin/.gitkeep` mantém a pasta no projeto.

## Comandos

Abrir a TUI:

```powershell
regieleki
```

Gerar a URL de download sem abrir a TUI:

```powershell
regieleki convert e75isJ7y
regieleki convert https://pixeldrain.com/u/e75isJ7y
```

O comando `convert` imprime a URL. O download automático acontece no fluxo interativo da TUI.

## Controles da TUI

- Digitar: informar o ID;
- `Enter`: iniciar o download;
- `Enter` após um erro: tentar novamente;
- `Enter` após concluir: começar outro download;
- `Esc` ou `Ctrl+C`: sair.

Durante o download, a interface mostra progresso, tamanho recebido, porcentagem quando o servidor informa o tamanho total, velocidade atual e destino.

## Comportamento dos arquivos

- O nome é obtido do header `Content-Disposition` quando disponível;
- Se não houver nome, o arquivo recebe `file_<id>`;
- Arquivos existentes não são sobrescritos: o programa cria nomes como `arquivo (1).zip`;
- Downloads incompletos são removidos quando ocorre uma falha.

## Testes

```bash
go test ./...
```

O endpoint usado pelo projeto está configurado em `workerBaseURL`, no arquivo `main.go`.
