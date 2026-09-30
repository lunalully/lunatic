# Contribuindo com o Lunatic

Obrigado por ajudar. O Lunatic é uma ferramenta de reconhecimento **estritamente passiva**; toda contribuição precisa preservar isso. As regras técnicas detalhadas para adaptadores estão em [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md); o panorama das fontes, em [docs/SOURCES.md](docs/SOURCES.md).

## Regras de contribuição (somente passivo)

Uma contribuição será recusada se fizer qualquer uma destas coisas:

- Contatar o domínio-alvo ou qualquer host descoberto (nem HTTP, nem TCP, nem DNS).
- Resolver DNS dos resultados, fazer força bruta de nomes ou varredura de portas.
- Disparar scans, jobs ou coletas sob demanda em provedores (use só endpoints de **consulta/busca de dados armazenados**; ex.: nunca `/scan/`, `/active/`, submissão de URLs).
- Seguir links encontrados nos resultados ou baixar páginas/arquivos arquivados.
- Contornar CAPTCHA, desafios anti-bot, limites de cota ou termos de uso de um provedor.
- Reter ou emitir dados pessoais (e-mails, contatos WHOIS, IPs). Apenas nomes de host.
- Usar dados provenientes de vazamentos ou infostealers (ver `hudsonrock`, desabilitada por esse motivo).
- Fazer requisições fora de `internal/httpx` (o único cliente HTTP permitido: guarda passiva, limite de taxa, tentativas, limite de tamanho, redação de segredos).

## Checklist de um novo adaptador

Um adaptador são **exatamente** estes arquivos:

- [ ] `internal/sources/<nome>.go`
- [ ] `internal/sources/<nome>_test.go`
- [ ] `internal/sources/testdata/<nome>/...` (fixtures)

E cumpre:

- [ ] Comentário de cabeçalho: provedor, link da doc, endpoint, tipo de autenticação, limites, data da consulta e situação da verificação (fixture / ao vivo).
- [ ] `var <nome>BaseURL = "..."` como variável de pacote (os testes a reatribuem).
- [ ] `func init() { Register(<nome>{}) }`; nome em minúsculas e único.
- [ ] `Info()` completo: `Name`, `URL`, `Auth`, `CredFields` (obrigatórios), `OptCredFields` (opcionais), `Default`, `RPS`, `Burst`. Fonte com chave obrigatória só é `Default` se a chave for realmente opcional para o uso padrão (o runner a pula sem chave). Provedor inviável: `Disabled: true` + `DisabledReason`, `Enumerate` retorna `ErrDisabled`.
- [ ] Toda requisição via `s.HTTP`; chaves em cabeçalho quando o provedor permitir; nunca logar segredos.
- [ ] Erros tipados (`ErrNoKey`, `ErrAuth`, `ErrRateLimited`, `ErrTimeout`, `ErrUnexpected`, `ErrUnavailable`, `ErrBlockedHost`, `ErrDisabled`). Falha de parse: `fmt.Errorf("%w: ...", ErrUnexpected)`. Zero resultados é sucesso (`nil`).
- [ ] Paginação limitada por `s.MaxPages` e por `ctx`; respeitar limites do provedor via `RPS`/`Burst`, sem `sleep` manual.
- [ ] Emitir nomes **crus** com `emit`; não normalizar, filtrar nem deduplicar (o runner faz isso).
- [ ] Documentar a fonte em `docs/SOURCES.md` (linha da tabela, com a situação de verificação honesta) e os campos de credencial em `config.example.yaml`.
- [ ] Não editar pacotes centrais nem outros adaptadores. Se o contrato for insuficiente, abra uma issue descrevendo a necessidade.

## Testes obrigatórios

Somente offline, com `httptest` e fixtures. **Nunca** acessar a rede real em testes unitários. Cubra:

- sucesso; resultado vazio; parada em `MaxPages`;
- 401 para `ErrAuth`; limite de taxa; JSON malformado para `ErrUnexpected`; chave ausente;
- que nenhuma rota fora do endpoint esperado é chamada (quando o provedor tem endpoints de scan).

Fixtures usam dados sintéticos (`example.com`), nunca dados reais de terceiros. Testes ao vivo são manuais e locais: use `scripts/live-smoke.sh` (só fontes padrão sem chave, uma por vez; ignora chaves do ambiente/config). Não os inclua na suíte.

## Estilo e verificações

Antes de abrir o PR:

```bash
gofmt -l .        # deve sair vazio
go vet ./...
go test ./...
go test -race ./...
go build ./cmd/lunatic
# ou tudo de uma vez:
make check
```

Mantenha o código simples, sem dependências novas sem discussão prévia (o projeto usa `vendor/` e evita dependências externas).

## Sem segredos nos commits

- Nunca faça commit de chaves de API, tokens, cookies ou arquivos `config.yaml` reais. O `config.example.yaml` só contém valores vazios.
- Fixtures e logs colados em issues não podem conter chaves. Revise antes de enviar.
- Se vazar uma chave, **revogue-a no provedor imediatamente**; apagar o commit não basta.

## Como reportar quebra de uma fonte

Provedores mudam de formato, cota e endpoint. Ao abrir uma issue:

1. Rode com `-v` e uma única fonte: `lunatic -d example.com -s <fonte> -v` (segredos são redigidos, mas confira).
2. Informe: nome da fonte, versão (`lunatic --version`), tipo da falha no resumo do `-v` (`auth`, `rate_limited`, `unexpected`, `unavailable`...) e o código de saída.
3. Se possível, descreva o formato novo da resposta com um **exemplo anonimizado** (sem chaves, sem dados pessoais).
4. Diga se o plano da sua chave inclui o endpoint.

Nunca inclua sua chave na issue.

## Licença

Ao contribuir, você concorda que sua contribuição será licenciada sob a licença MIT do projeto ([LICENSE](LICENSE)). Não copie código de outros projetos sem verificar a licença e atribuir adequadamente; o Subfinder, por exemplo, serve aqui apenas como referência de endpoints, sem cópia de código.
