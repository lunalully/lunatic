# Lunatic

```
                                                           lun
  ▓     ▓   ▓ ▓   ▓  ▓▓▓  ▓▓▓▓▓ ▓▓▓▓▓  ▓▓▓▓               aticl
  ▓     ▓   ▓ ▓▓  ▓ ▓   ▓   ▓     ▓   ▓                  unaticl
  ▓     ▓   ▓ ▓▓  ▓ ▓   ▓   ▓     ▓   ▓                 unat iclu
  ▓     ▓   ▓ ▓ ▓ ▓ ▓▓▓▓▓   ▓     ▓   ▓        naticlunatic   lunaticlunat
  ▓     ▓   ▓ ▓  ▓▓ ▓   ▓   ▓     ▓   ▓        icluna               ticlun
  ▓     ▓   ▓ ▓  ▓▓ ▓   ▓   ▓     ▓   ▓           aticl           unati
  ▓▓▓▓▓  ▓▓▓  ▓   ▓ ▓   ▓   ▓   ▓▓▓▓▓  ▓▓▓▓          clun       atic
                                                     lun    a    tic
                                                     lun aticlun ati
  Passive Subdomain Recon  v0.1.0                   clunati   clunati
                                                   clunat       icluna
```

**Lunatic** é uma ferramenta de código aberto, escrita em Go, para descoberta de subdomínios **estritamente passiva**. Ela consulta apenas provedores de dados de terceiros (logs de Certificate Transparency, arquivos web, bases de DNS passivo, mecanismos de busca de ativos etc.). Ela **nunca** faz varredura, força bruta, resolução de DNS ou requisição HTTP ao alvo.

## O que o Lunatic descobre (e o que não garante)

O resultado é uma lista de **nomes observados em fontes passivas**. Isso **não é prova** de que o nome:

- está online ou responde hoje;
- resolve em DNS;
- pertence de fato à organização (curingas, SANs de certificados compartilhados e dados comunitários trazem ruído);
- ainda existe: Certificate Transparency e arquivos web guardam **registros históricos**, então nomes antigos e já desativados aparecem.

Trate a saída como ponto de partida para investigação, não como inventário confirmado.

## Instalação (Linux / Kali)

Requisitos: **Go 1.24 ou superior** (linha `go 1.24` do `go.mod`). As dependências (`golang.org/x/net`, `golang.org/x/text`) já vêm em `vendor/`; não é preciso acesso à rede para compilar.

Instale o Go pelo [go.dev/dl](https://go.dev/dl/) ou, no Kali/Debian, por `apt`:

```bash
sudo apt update && sudo apt install golang-go
go version    # confira se é 1.24 ou superior; se for mais antigo, use o go.dev
```

> O pacote `golang-go` de algumas versões do Kali/Debian pode ser mais antigo que 1.24. Nesse caso instale o Go pelo go.dev.

Baixe o código e compile:

```bash
git clone <URL do seu repositório> lunatic
cd lunatic

make build        # gera ./bin/lunatic
make install      # compila e instala em /usr/local/bin (ou ~/go/bin se não houver permissão)

# alternativa sem make:
go build -o lunatic ./cmd/lunatic
```

`make install` aceita `PREFIX=/outro/caminho`. Se instalou em `~/go/bin`, garanta que ele esteja no `PATH`.

## Primeiro uso (sem configuração)

Funciona sem nenhuma chave de API. Só as fontes gratuitas rodam:

```bash
lunatic -d example.com
```

Use apenas em domínios que você tem autorização para investigar. A consulta é passiva, mas o resultado é informação sobre um alvo.

## Exemplos

```bash
lunatic -d example.com                                   # fontes padrão, resultado no terminal
lunatic -d example.com -o subdominios.txt                # também grava em arquivo (texto)
lunatic -dL dominios.txt -o resultados.txt               # lista de domínios (um por linha)
lunatic -d example.com -s crtsh,commoncrawl,waybackarchive   # somente estas fontes
lunatic -d example.com --all                             # todas as fontes não desabilitadas
lunatic -d example.com --json -o resultados.jsonl        # JSON Lines (só domain e subdomain)
lunatic -d example.com --json --show-sources             # JSON Lines com a lista de fontes por nome
lunatic -d example.com -v                                # mostra o resumo por fonte (ok/skipped/failed)
lunatic -d example.com --silent                          # só os resultados, ideal para pipes
lunatic --list-sources                                   # tabela de fontes, status e variáveis
lunatic --help
lunatic --version
```

`-d` pode ser repetido ou receber lista separada por vírgulas (`-d a.com,b.com`). Em `-dL`, linhas vazias e comentários iniciados por `#` são ignorados.

## Flags

Valores padrão conforme `internal/cli/cli.go`.

| Flag | Descrição | Padrão |
|---|---|---|
| `-d domínio` | Domínio alvo (repetível ou separado por vírgulas) | - |
| `-dL arquivo` | Arquivo com um domínio por linha (`#` comenta) | - |
| `-s lista` | Roda apenas estas fontes (nomes separados por vírgula) | fontes padrão |
| `-es`, `--exclude-sources lista` | Pula estas fontes | nenhuma |
| `--all` | Usa todas as fontes não desabilitadas (as sem chave aparecem como "skipped" com `-v`). Não combina com `-s` | desligado |
| `--list-sources` | Imprime a tabela de fontes e sai | - |
| `-o arquivo` | **Também** grava os resultados no arquivo (criado/truncado, permissão 0644) | - |
| `--json`, `-oJ` | Saída JSON Lines | texto |
| `--show-sources` | Com `--json`, inclui o array `sources` em cada registro (o texto sempre traz só os nomes) | desligado |
| `--silent` | Só resultados no stdout; também esconde o banner (e ignora `-v`) | desligado |
| `--config caminho` | Arquivo YAML de credenciais | `$XDG_CONFIG_HOME/lunatic/config.yaml` ou `~/.config/lunatic/config.yaml` |
| `--timeout segundos` | Tempo limite **por fonte** e por domínio (mínimo 1) | `90` (segundos) |
| `--max-time minutos` | Tempo limite total da execução; `0` = sem limite | `10` (minutos) |
| `--concurrency n` | Fontes rodando ao mesmo tempo (mínimo 1) | `10` |
| `-v`, `--verbose` | Mostra progresso, avisos de configuração e o resumo por fonte (ok/skipped/failed com motivos) no stderr; segredos redigidos. Sem `-v` o stderr mostra só o banner | desligado |
| `--no-color` | Desativa cores (também: `NO_COLOR`, `TERM=dumb`, stderr fora de TTY) | cores automáticas |
| `--version` | Imprime a versão e sai | - |
| `-h`, `--help` | Mostra a ajuda | - |

Nomes de fontes desconhecidos em `-s` ou `--exclude-sources` geram erro de uso (código 1) com a lista de nomes válidos.

## Configuração de APIs

Muitas fontes exigem (ou aceitam opcionalmente) uma chave. A configuração vem de um arquivo YAML e/ou de variáveis de ambiente.

**Precedência: variável de ambiente > arquivo.**

- Caminho padrão: `$XDG_CONFIG_HOME/lunatic/config.yaml`, ou `~/.config/lunatic/config.yaml`. Um arquivo padrão ausente não é erro; um `--config` explícito que não pode ser lido é erro (código 1).
- Variável de ambiente: `LUNATIC_<FONTE>_<CAMPO>`, em maiúsculas (o que não for letra/número vira `_`). Ex.: `LUNATIC_SHODAN_API_KEY`, `LUNATIC_FOFA_EMAIL`.
- Valores vazios e placeholders (`<...>`, `YOUR_...`, `CHANGEME`, `TODO`, `NONE`, `NULL`) contam como **sem chave**.
- `lunatic --list-sources` mostra, para cada fonte, os nomes exatos das variáveis.

```bash
mkdir -p ~/.config/lunatic
cp config.example.yaml ~/.config/lunatic/config.yaml
chmod 600 ~/.config/lunatic/config.yaml    # o Lunatic avisa se o arquivo for legível por todos
```

Formato (um subconjunto simples de YAML; use espaços, não tabs):

```yaml
sources:
  shodan:
    api_key: "SUA_CHAVE"
  dnsrepo:                # fonte com vários campos obrigatórios
    apikey: "SUA_CHAVE"
    token: "SEU_TOKEN"
  fofa:                   # key obrigatória; email opcional
    key: "SUA_CHAVE"
    email: ""
  intelx:                 # api_key obrigatória; host opcional (padrão free.intelx.io)
    api_key: "SUA_CHAVE"
    host: "2.intelx.io"
```

Ou só por ambiente, sem arquivo:

```bash
export LUNATIC_VIRUSTOTAL_API_KEY="..."
lunatic -d example.com
```

**Nunca faça commit de chaves reais.** O `config.example.yaml` lista todas as fontes com campos de credencial, com valores vazios. Campos **opcionais** (ex.: `fofa.email`, `intelx.host`, `zoomeyeapi.host`) nunca impedem a fonte de rodar; campos **obrigatórios** ausentes fazem a fonte ser pulada.

## Fontes gratuitas vs. com restrição

O que roda em cada modo (a lista completa e o estado real ficam em `lunatic --list-sources`):

- **Execução padrão** (sem `-s` nem `--all`): fontes marcadas como padrão que estejam utilizáveis.
  - **Gratuitas, sem chave**: anubis, crtsh, scanmalware, shodanct, subdomaincenter, thc, waybackarchive.
  - **Chave opcional** (rodam sem ela; a chave amplia a cota): alienvault, certspotter, hackertarget, submd, urlscan.
  - **Com chave obrigatória**: entram na execução padrão **somente se a chave estiver configurada**; sem ela aparecem como "needs key" e são puladas.
- **Não-padrão** (só com `-s nome` ou `--all`): commoncrawl, digitorus, rapiddns, sitedossier (gratuitas, mas pesadas, instáveis ou baseadas em raspagem de HTML), e threatbook e zoomeyeapi (exigem chave).
- **`--all`**: todas as fontes não desabilitadas. As que precisam de chave e não a têm aparecem como `skipped (missing credentials: ...)` no resumo do `-v`.
- **Desabilitadas** (binaryedge, censys, chinaz, domainsproject, hudsonrock, robtex, threatcrowd): registradas para cobertura, nunca contatam a rede, mesmo com `-s`. Motivos em [docs/SOURCES.md](docs/SOURCES.md).

Detalhes por fonte (endpoint, plano, limitações, situação de verificação): **[docs/SOURCES.md](docs/SOURCES.md)**.

## Formatos de saída

Os resultados vão **sempre para o stdout**. Com `-o arquivo`, os mesmos dados são **também** gravados no arquivo (criado ou truncado), no formato escolhido (`--json` ou texto). Nunca há ANSI, banner ou resumo nos dados. A escrita acontece ao final da execução, não em fluxo contínuo.

**Texto (padrão)**: um subdomínio por linha, sem repetições.

```
api.example.com
www.example.com
```

**JSON Lines (`--json`)**: um objeto por linha, apenas com `domain` e `subdomain` (sem nomes de fontes).

```json
{"domain":"example.com","subdomain":"api.example.com"}
{"domain":"example.com","subdomain":"www.example.com"}
```

Com `--json --show-sources`, cada registro ganha o array `sources` com as fontes que observaram o nome (a saída de texto nunca inclui fontes):

```json
{"domain":"example.com","subdomain":"api.example.com","sources":["crtsh","waybackarchive"]}
```

## stdout e stderr

| Fluxo | Conteúdo |
|---|---|
| stdout | Apenas os resultados (dados) |
| stderr | Só o banner (apenas se stderr for um terminal) e erros fatais de uso/configuração. Nenhum nome de fonte, falha, aviso ou contagem aparece por padrão; com `-v` surgem o progresso, os avisos e o resumo por fonte |

Com `--silent`, o stderr só recebe erros fatais (sem banner). Assim `lunatic ... > saida.txt` e `| outro-programa` recebem só dados.

## Códigos de saída

| Código | Significado |
|---|---|
| `0` | Todas as fontes tentadas tiveram sucesso (zero resultados conta como sucesso) |
| `1` | Erro de uso, entrada ou configuração (nada foi executado) |
| `2` | Nenhuma fonte teve sucesso (todas falharam, foram puladas ou nenhuma era utilizável) |
| `3` | Sucesso parcial: ao menos uma fonte teve sucesso e ao menos uma falhou |
| `130` | Interrompido (SIGINT/SIGTERM); resultados parciais ainda são escritos |

Fontes **puladas** (sem chave, desabilitadas) não contam como falha para o código `3`.

A saída é limpa por padrão (nada sobre fontes no stderr), então o **código de saída** é o sinal de que alguma fonte falhou (`3` parcial, `2` nenhuma teve sucesso). Rode com `-v` para ver quais.

## Interpretando falhas, resultados vazios e registros históricos

Por padrão o Lunatic não imprime nada sobre fontes. Para saber quais fontes falharam, foram puladas ou funcionaram, **use `-v`**: o resumo (stderr) lista cada fonte como `ok N`, `skipped (motivo)` ou `failed (tipo): mensagem`. Tipos de falha: `no_key`, `auth` (chave inválida ou plano sem acesso), `rate_limited` (cota/limite de taxa), `timeout`, `unexpected` (resposta em formato inesperado, provedor mudou), `unavailable` (5xx, desafio anti-bot), `blocked` (guarda passiva), `canceled`.

- **Resultado vazio não é erro.** Um domínio pequeno pode simplesmente não aparecer nas fontes.
- **Falha parcial (código 3)** é comum: fontes gratuitas como crt.sh oscilam. Rode de novo com `-v` para ver quais fontes falharam e por quê, e aumente `--timeout` se necessário.
- **`auth`** costuma indicar chave inválida ou plano que não inclui o endpoint. **`unexpected`** pode indicar que o provedor mudou o formato: veja a seção de contribuição para reportar.
- **Nomes históricos**: CT e arquivos web guardam registros antigos. Um nome listado pode ter sido desativado anos atrás.
- Serviços limitam a cota gratuita e podem truncar resultados sem avisar (ver [docs/SOURCES.md](docs/SOURCES.md)).

## Limites passivos (garantias de projeto)

- **Sem força bruta** de nomes.
- **Sem resolução de DNS** dos resultados.
- **Sem HTTP para o alvo**: todo acesso de rede passa por um único cliente (`internal/httpx`) que **recusa** qualquer requisição ao domínio-alvo ou seus subdomínios.
- **Guarda de redirecionamento**: redirecionamentos para o alvo são bloqueados; no máximo 3 saltos; só para o mesmo host (ou http para https no mesmo host).
- Nunca segue links encontrados nos resultados, nunca dispara jobs/scans nos provedores, e guarda apenas nomes (sem e-mails, IPs ou dados pessoais).
- Fontes de raspagem (digitorus, rapiddns, sitedossier) nunca contornam CAPTCHA ou desafios anti-bot.

## Integração com pipes

Com `--silent` a saída é só dados:

```bash
lunatic -d example.com --silent | sort -u > subs.txt
lunatic -d example.com --silent | httpx
```

> **Atenção:** o Lunatic é passivo, mas ferramentas como **httpx**, **dnsx**, nmap etc. fazem **verificação ATIVA** contra os alvos (resolvem DNS e enviam requisições aos hosts). Só as use em alvos para os quais você tem **autorização explícita**. Encadear no pipe torna a etapa seguinte ativa; a responsabilidade é sua.

## Como adicionar e testar uma nova fonte (adaptador)

Resumo; as regras completas estão em [AGENTS.md](AGENTS.md) e [CONTRIBUTING.md](CONTRIBUTING.md).

1. Crie exatamente estes arquivos: `internal/sources/<nome>.go`, `internal/sources/<nome>_test.go` e fixtures em `internal/sources/testdata/<nome>/`.
2. Declare a URL base como **variável de pacote** (os testes a reatribuem): `var nomeBaseURL = "https://api.exemplo.com"`.
3. Registre no `init()`: `func init() { Register(nome{}) }`. Nomes são minúsculos e únicos (duplicado causa panic).
4. Preencha `Info()`: `Name`, `URL`, `Auth` (`AuthNone`, `AuthOptional`, `AuthRequired`), `CredFields` (obrigatórios) e `OptCredFields` (opcionais), `Default`, `RPS`, `Burst`. Fonte que exige chave só deve ser `Default` se a chave for opcional (a execução padrão a pula sem chave). Fontes inviáveis: `Disabled: true` com `DisabledReason`, e `Enumerate` retorna `ErrDisabled`.
5. Faça todas as requisições por `s.HTTP` (nunca `net/http` direto). Retorne erros tipados (`ErrNoKey`, `ErrAuth`, `ErrRateLimited`, `ErrTimeout`, `ErrUnexpected`, `ErrUnavailable`, `ErrBlockedHost`, `ErrDisabled`). Credenciais via `PickKey(s.Creds, "api_key")`. Emita nomes crus com `emit(...)`; o runner normaliza, filtra o escopo e remove duplicatas.
6. Testes offline com `httptest` e fixtures: sucesso, resultado vazio, parada em `MaxPages`, 401 para `ErrAuth`, limite de taxa, JSON malformado para `ErrUnexpected` e chave ausente. **Nunca** acesse a rede em testes.
7. Rode `make test` (e `make check` antes de abrir PR: `go vet`, testes, `-race`, `gofmt`).
8. Documente a fonte em `docs/SOURCES.md` e, se houver credenciais, em `config.example.yaml`.

## Sobre esta versão: testes ao vivo

Nesta build **não foi possível testar contra os provedores reais**: o ambiente de construção só tinha egress por proxy que bloqueava os hosts (403 em `CONNECT`). Todas as fontes têm testes com fixture, mas **nenhuma foi confirmada ao vivo**, e vários formatos de resposta seguem a documentação e referências de terceiros. Rode localmente `scripts/live-smoke.sh` (teste de fumaça com rede real) e reporte divergências. Lista dos pontos mais incertos em [docs/SOURCES.md](docs/SOURCES.md).

## Licença

MIT. Veja [LICENSE](LICENSE). Copyright (c) 2026 Lunatic contributors.

**Avisos de terceiros (NOTICE):**

- O diretório `vendor/` inclui `golang.org/x/net` e `golang.org/x/text`, licenciados sob BSD-3-Clause, Copyright The Go Authors. O texto da licença acompanha cada módulo em `vendor/`.
- Nenhum código do **Subfinder** foi copiado. O código do Subfinder (MIT, ProjectDiscovery) foi usado apenas como **referência** para localizar endpoints de provedores. Todos os adaptadores foram escritos de forma independente.
