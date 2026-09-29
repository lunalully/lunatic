# Fontes do Lunatic

Referência de todas as fontes passivas: método, credenciais, situação e limitações. Data da consulta às documentações: **2026-09-29**.

> **Honestidade primeiro.** Todas as fontes implementadas têm apenas **teste com fixture** (servidor `httptest` local com dados sintéticos). **Nenhuma foi confirmada ao vivo**: o ambiente de build só tinha egress via proxy que respondia 403 a `CONNECT` para todos os provedores. Nomes de campos e caminhos de várias fontes vêm da documentação e/ou de referências de terceiros (Subfinder, BBOT, theHarvester), não de respostas reais. Veja a seção "Endpoints a validar na primeira execução ao vivo". Use `scripts/live-smoke.sh` localmente para validar.

O **código é a fonte da verdade**: cada linha abaixo foi conferida contra o `Info()` do adaptador (Auth, Default, Disabled, CredFields, OptCredFields). Para ver o estado real no seu ambiente: `lunatic --list-sources`.

## Sobre os símbolos `*` e `~` da lista original

A lista original de fontes marcava algumas com `*` e `~`. Os pesquisadores **não conseguiram confirmar nenhuma legenda oficial** (README e docs do Subfinder não trazem uma). A leitura mais bem sustentada é:

- `*` = **exige chave de API**;
- `~` = **chave opcional** (ou camada gratuita).

Isso é uma **inferência não confirmada**. A situação que vale é a coluna "Situação" abaixo, derivada do código.

## Legenda de situação

| Situação | Significado |
|---|---|
| implementada padrão | Roda na execução padrão (sem `-s`/`--all`). Se aceita chave opcional, funciona sem ela. |
| implementada com chave | Exige credencial. Entra na execução padrão **somente** se a credencial estiver configurada; sem ela aparece como "needs key" e é pulada. |
| implementada não-padrão | Não roda por padrão (`Default: false`). Use `-s nome` ou `--all`. Se também exigir chave (threatbook, zoomeyeapi), `--all` a pula quando faltar credencial. |
| desabilitada | Registrada só para listagem/cobertura; nunca contata a rede. Ignorada por `--all`; `-s nome` a lista como `skipped (motivo)` no resumo. |

Verificação: **fixture ✓** = teste offline passa; **ao vivo ✗** = não confirmado contra o provedor real.

## Tabela de fontes

| Identificador + link oficial | Método passivo e endpoint | Credenciais / restrições de plano (+ env vars) | Situação | Limitações conhecidas | Data da consulta | Verificação | Motivo do impedimento |
|---|---|---|---|---|---|---|---|
| **alienvault**<br>https://otx.alienvault.com | GET /api/v1/indicators/domain/{d}/passive_dns | Chave opcional (cabeçalho X-OTX-API-KEY; conta gratuita). `LUNATIC_ALIENVAULT_API_KEY` | implementada padrão | Sem paginação; cabeçalho de auth não confirmado ao vivo (Subfinder usa Bearer) | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado (proxy 403 em CONNECT) |
| **anubis**<br>https://anubisdb.com | GET /anubis/subdomains/{d} (o POST de envio nunca é usado) | Nenhuma; 60 req/10 s por IP | implementada padrão | Dado comunitário, pode ter lixo; 403 ("entrada inválida") vira erro de autenticação | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado (proxy 403 em CONNECT) |
| **bevigil**<br>https://bevigil.com/osint-api | GET osint.bevigil.com/api/{d}/subdomains/ | Obrigatória, cabeçalho X-Access-Token; chave gratuita, créditos não confirmados. `LUNATIC_BEVIGIL_API_KEY` | implementada com chave | Caminho vem só do Subfinder; portal de docs ilegível | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **bufferover**<br>https://tls.bufferover.run | GET /dns?q=.{d} | Obrigatória, cabeçalho x-api-key; gratuito ~100 req/mês, uso não comercial. `LUNATIC_BUFFEROVER_API_KEY` | implementada com chave | Instabilidade relatada; formato das strings não confirmado (hostnames extraídos por regex) | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **builtwith**<br>https://api.builtwith.com | GET /v26/api.json?KEY&LOOKUP&HIDETEXT=yes&NOMETA=yes&NOPII=yes | Obrigatória, chave na query string; Domain API paga. `LUNATIC_BUILTWITH_API_KEY` | implementada com chave | v26 vem da doc (Subfinder usa v21); custo em créditos desconhecido; chave na URL depende da redação do httpx | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **c99**<br>https://api.c99.nl | GET api.c99.nl/subdomainfinder?key&domain&json (nenhuma opção de scan/tempo real é enviada) | Obrigatória, chave na query string; plano pago. `LUNATIC_C99_API_KEY` | implementada com chave | O fornecedor descreve o endpoint como "advanced scan": comportamento só de dados armazenados não confirmado; IPs da resposta são descartados | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **censys**<br>https://platform.censys.io | Nenhum (não chama a rede) | Platform API: token pessoal + `X-Organization-ID`. `LUNATIC_CENSYS_API_TOKEN`, `LUNATIC_CENSYS_ORGANIZATION_ID` (reservados) | desabilitada | Ver seção de desabilitadas | 2026-09-29 | Info/ErrDisabled ✓ / n/a | Desabilitada (ver seção abaixo) |
| **certspotter**<br>https://sslmate.com/certspotter | GET /v1/issuances?domain&include_subdomains=true&expand=dns_names&after=&lt;id&gt; | Chave opcional (Bearer); sem chave há cota horária pequena. `LUNATIC_CERTSPOTTER_API_KEY` | implementada padrão | 429 esperado sem chave; paginação limitada por MaxPages; SANs curinga/alheios chegam crus e são filtrados pelo runner | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado (proxy 403 em CONNECT) |
| **chaos**<br>https://chaos.projectdiscovery.io | GET dns.projectdiscovery.io/dns/{d}/subdomains | Obrigatória, chave crua em Authorization (sem Bearer). `LUNATIC_CHAOS_API_KEY` | implementada com chave | Limites e existência de plano gratuito não confirmados | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **chinaz**<br>https://api.chinaz.com | Nenhum (não chama a rede) | Chave (100 consultas gratuitas). `LUNATIC_CHINAZ_API_KEY` (reservada) | desabilitada | Ver seção de desabilitadas | 2026-09-29 | Info/ErrDisabled ✓ / n/a | Desabilitada (ver seção abaixo) |
| **commoncrawl**<br>https://commoncrawl.org | GET /collinfo.json e depois, nos 3 índices mais recentes, /{id}-index?url=*.{d}&output=json&fl=url (só índice, sem WARC) | Nenhuma | implementada não-padrão | Limite de taxa agressivo (503 aborta a fonte); MaxPages vale por índice; forma de `showNumPages` vem da doc do PyWB, não testada ao vivo | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado (proxy 403 em CONNECT) |
| **crtsh**<br>https://crt.sh | GET /?q=%25.{d}&output=json (HTTP JSON; sem PostgreSQL) | Nenhuma | implementada padrão | Lento; 502/503/timeouts frequentes; HTML com status 200 vira erro inesperado; e-mails em SAN são descartados | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado (proxy 403 em CONNECT) |
| **digitalyama**<br>https://digitalyama.com | GET api.digitalyama.com/subdomain_finder?domain= | Obrigatória, cabeçalho x-api-key; 25 créditos gratuitos, 1 crédito/chamada, 1 req/s. `LUNATIC_DIGITALYAMA_API_KEY` | implementada com chave | Caminho vem só do Subfinder; não se sabe se o provedor faz DNS ao vivo; código ao esgotar créditos não confirmado | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **digitorus**<br>https://certificatedetails.com | GET certificatedetails.com/{d} (HTML de logs CT; extração de texto) | Nenhuma | implementada não-padrão | Sem contrato de API (raspagem, pode quebrar); desafio anti-bot/403/503 vira indisponível (nunca contornado); sem ToS localizado | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado (proxy 403 em CONNECT) |
| **dnsdb**<br>https://www.domaintools.com/products/dnsdb/ | GET api.dnsdb.info/dnsdb/v2/lookup/rrset/name/*.{d}?limit&offset (NDJSON) | Obrigatória, X-API-Key; paga, cotas por chave. `LUNATIC_DNSDB_API_KEY` | implementada com chave | Nomes históricos, incl. curingas; paginação por offset depende de `offset_max` da chave; consultas com curinga à esquerda são custosas | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **dnsdumpster**<br>https://dnsdumpster.com | GET api.dnsdumpster.com/domain/{d} (hosts a/cname/mx/ns) | Obrigatória, X-API-Key; chave gratuita: 50 req/dia, 50 registros/domínio, 1 req/2 s. `LUNATIC_DNSDUMPSTER_API_KEY` | implementada com chave | Só a página 1 (paginação não documentada); teto gratuito trunca; docs não dizem se a consulta é só de dados armazenados | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **dnsrepo**<br>https://dnsarchive.net (antigo DNSRepo) | GET dnsarchive.net/api/?apikey=&search={d}&page&limit=500 + cabeçalho X-API-Access | Obrigatória: apikey (query) + token (cabeçalho); paga (~US$ 20/mês). `LUNATIC_DNSREPO_APIKEY`, `LUNATIC_DNSREPO_TOKEN` | implementada com chave | Esquema da resposta vem de código de terceiros (array de {domain}); busca em toda a base, filtrada por escopo no runner | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **domainsproject**<br>https://domainsproject.org | Nenhum (não chama a rede) | Só por aprovação. `LUNATIC_DOMAINSPROJECT_USERNAME`, `LUNATIC_DOMAINSPROJECT_PASSWORD` (reservados) | desabilitada | Ver seção de desabilitadas | 2026-09-29 | Info/ErrDisabled ✓ / n/a | Desabilitada (ver seção abaixo) |
| **driftnet**<br>https://driftnet.io | GET api.driftnet.io/v1/{ct/log,scan/protocols,scan/domains,domain/rdns}?field=host:{d}&summarize=host&summary_context=..&summary_limit=10000 | Obrigatória, Bearer; cota comunitária gratuita por solicitação. `LUNATIC_DRIFTNET_API_KEY` | implementada com chave | Valores de `summary_context` vêm de código de terceiros; endpoint que responde 4xx é pulado (erro só se todos falharem); até 4 requisições/domínio consomem cota | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **fofa**<br>https://fofa.info | GET fofa.info/api/v1/search/all?key&qbase64=domain="{d}"&fields=host&size=1000&page | `key` obrigatória (query); `email` opcional, enviado só se configurado. `LUNATIC_FOFA_KEY`, `LUNATIC_FOFA_EMAIL` (opcional) | implementada com chave | `domain=` também casa outros domínios/IPs (runner filtra); erros mapeados por heurística de texto; custo em F-coin e limites do plano gratuito não verificados | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **fullhunt**<br>https://fullhunt.io | GET fullhunt.io/api/v1/domain/{d}/subdomains (endpoints de scan nunca chamados) | Obrigatória, X-API-KEY; 200 req/h. `LUNATIC_FULLHUNT_API_KEY` | implementada com chave | O plano trunca resultados em silêncio; 402 vira limite de taxa | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **github**<br>https://github.com | GET api.github.com/search/code?q="{d}"&per_page=100&page=N (nomes extraídos só dos fragmentos) | Obrigatória, token pessoal (Bearer); busca de código: 10 req/min, máx. 1000 resultados. `LUNATIC_GITHUB_TOKEN` | implementada com chave | Fragmentos curtos (baixo rendimento); nunca baixa arquivos/repositórios; 422 na página seguinte encerra a paginação | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **hackertarget**<br>https://hackertarget.com | GET api.hackertarget.com/hostsearch/?q={d} (CSV host,ip; só o host é usado) | Chave opcional (X-API-Key); gratuito ~50 chamadas/dia por IP (docs divergem: 20). `LUNATIC_HACKERTARGET_API_KEY` | implementada padrão | Erro/cota chega com HTTP 200 (mapeado por texto); gratuito trunca em ~50; IPs compartilhados esgotam a cota rápido | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado (proxy 403 em CONNECT) |
| **hudsonrock**<br>https://www.hudsonrock.com | Nenhum (não chama a rede) | n/a | desabilitada | Ver seção de desabilitadas | 2026-09-29 | Info/ErrDisabled ✓ / n/a | Desabilitada (ver seção abaixo) |
| **intelx**<br>https://intelx.io | POST {host}/phonebook/search?k=KEY e depois GET /phonebook/search/result?k&id (seletores; só hostnames) | `api_key` obrigatória; `host` opcional (padrão free.intelx.io; deve casar com o plano da chave). `LUNATIC_INTELX_API_KEY`, `LUNATIC_INTELX_HOST` (opcional) | implementada com chave | Integrações de terceiros exigem licença de API; e-mails/URLs descartados; endpoints de arquivos/buckets nunca chamados; cota gratuita não verificada | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **leakix**<br>https://leakix.net | GET leakix.net/api/subdomains/{d} | Obrigatória, cabeçalho api-key; gratuito (3000 req, resultados atrasados ~15 dias). `LUNATIC_LEAKIX_API_KEY` | implementada com chave | Sem paginação documentada | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **merklemap**<br>https://www.merklemap.com/documentation/search | GET api.merklemap.com/v1/search?query=*.{d}&page=N (0-indexado) | Obrigatória, Bearer; plano pago (~EUR 49/mês), sem plano gratuito confirmado. `LUNATIC_MERKLEMAP_API_KEY` | implementada com chave | Tamanho de página desconhecido; para em página vazia ou contagem atingida | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **netlas**<br>https://docs.netlas.io/api-reference/ | GET /api/domains_count/?q=domain:*.{d} AND NOT domain:{d} e depois POST /api/domains/download/ | Obrigatória, Bearer; plano Community gratuito, não comercial, ~50 req/dia. `LUNATIC_NETLAS_API_KEY` | implementada com chave | Corpo do download vem do Subfinder (não confirmado na doc oficial); download limitado a 200 resultados; 2 requisições por execução | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **onyphe**<br>https://search.onyphe.io/docs/general-apis/search | GET www.onyphe.io/api/v2/search/?q=category:resolver domain:{d}&page=N&size=1000 (só busca v2 em dados armazenados; a v3 sob demanda nunca é usada) | Obrigatória, `Authorization: bearer`; planos pagos, sem plano gratuito confirmado. `LUNATIC_ONYPHE_API_KEY` | implementada com chave | Categoria `resolver` vem do Subfinder; campos string ou array (tratados); teto de 10.000 resultados | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **profundis**<br>https://docs.profundis.io/api/api-usage | POST api.profundis.io/api/v2/common/data/subdomains {domain} (resposta em stream) | Obrigatória, X-API-KEY; assinatura/créditos (cada chamada custa créditos). `LUNATIC_PROFUNDIS_API_KEY` | implementada com chave | Endpoint vem do Subfinder (docs listam só /hosts e /dns); prefixos SSE `data:` tolerados | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **pugrecon**<br>https://gist.github.com/c3l3si4n/68ac06ebe85f8c0b821800432c7f89a6 | POST pugrecon.com/api/v1/domains {domain_name} | Obrigatória, Bearer; cota gratuita minúscula (docs divergem: 20 ou 50/mês). `LUNATIC_PUGRECON_API_KEY` | implementada com chave | Sem paginação; plano gratuito trunca (`limited`); mensagem de cota com resultado vazio vira limite de taxa | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **quake**<br>https://quake.360.net | POST quake.360.net/api/v3/search/quake_service {query: domain:"{d}", include:[service.http.host], latest:true, size:100} | Obrigatória, X-QuakeToken; consome pontos de cota por resultado; conta pode exigir verificação. `LUNATIC_QUAKE_API_KEY` | implementada com chave | Endpoint vem do Subfinder; `code` diferente de zero mapeado por texto de mensagem | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **rapiddns**<br>https://rapiddns.io | GET rapiddns.io/subdomain/{d}?page=N&full=1 (HTML público) | Nenhuma | implementada não-padrão | Raspagem não oficial (referência do Subfinder); ToS não lido; CAPTCHA/desafio/403 vira indisponível (nunca contornado); API Pro JSON não implementada | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado (proxy 403 em CONNECT) |
| **reconeer**<br>https://www.reconeer.com/docs.html | GET www.reconeer.com/api/domain/{d} (dados em cache; 404 = nenhum) | Obrigatória, Bearer; gratuito 10 consultas/dia (docs dizem que anônimo funciona, mas 401 foi observado). `LUNATIC_RECONEER_API_KEY` | implementada com chave | 402 vira erro de autenticação (plano); sem paginação | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **redhuntlabs**<br>https://redhuntlabs.com/attack-surface-recon-api/ | GET reconapi.redhuntlabs.com/community/v1/domains/subdomains?domain={d}&page=N&page_size=1000 | Obrigatória, X-BLOBR-KEY; plano community gratuito: 100 req/mês. `LUNATIC_REDHUNTLABS_API_KEY` | implementada com chave | No máximo 3 páginas (cota); só o prefixo community (outros planos usam outro prefixo, não configurável) | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **robtex**<br>https://www.robtex.com/api/ | Nenhum (não chama a rede) | n/a | desabilitada | Ver seção de desabilitadas | 2026-09-29 | Info/ErrDisabled ✓ / n/a | Desabilitada (ver seção abaixo) |
| **rsecloud**<br>https://rsecloud.com | GET api.rsecloud.com/api/v2/subdomains/passive/{d}?page=N (nunca /active/) | Obrigatória, X-API-Key; limites desconhecidos. `LUNATIC_RSECLOUD_API_KEY` | implementada com chave | Endpoint vem do Subfinder; semântica de /active/ desconhecida, por isso nunca chamado | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **scanmalware**<br>https://scanmalware.com/docs | GET scanmalware.com/api/v1/ct/dns/{d}?subdomain_limit=5000 (consulta CT/pDNS; sem endpoint de scan/envio) | Nenhuma (anônimo: 600/min); chave opcional não suportada (nome do cabeçalho não confirmado) | implementada padrão | NOMES DE CAMPO NÃO CONFIRMADOS: o parser percorre todas as strings do JSON; flag de truncamento ignorada; padrão por decisão de projeto apesar de sem teste ao vivo | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado (proxy 403 em CONNECT) |
| **securitytrails**<br>https://docs.securitytrails.com/reference/list-subdomains-old-1 | GET api.securitytrails.com/v1/domain/{d}/subdomains?children_only=false&include_inactive=true | Obrigatória, cabeçalho APIKEY; paga (acesso gratuito à API não confirmado). `LUNATIC_SECURITYTRAILS_API_KEY` | implementada com chave | Requisição única; consome cota mensal; DSL/scroll não usado | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **shodan**<br>https://developer.shodan.io/api | GET api.shodan.io/dns/domain/{d}?key=&page=N | Obrigatória, chave na query; o endpoint de DNS provavelmente exige plano pago/membership. `LUNATIC_SHODAN_API_KEY` | implementada com chave | 1 crédito por página; 404 tratado como sem resultados (não confirmado) | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **shodanct**<br>https://ctl.shodan.io | GET ctl.shodan.io/api/v1/domain/{d}/hostnames (array JSON) | Nenhuma observada; limites não documentados | implementada padrão | Docs silenciam sobre limites/ToS; endpoint deduzido do site e do Subfinder | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado (proxy 403 em CONNECT) |
| **sitedossier**<br>http://www.sitedossier.com | GET www.sitedossier.com/parentdomain/{d} e links de "próxima" (HTML) | Nenhuma | implementada não-padrão | Raspagem em HTTP simples, sem ToS/limites; CAPTCHA/bloqueio vira indisponível (nunca contornado) | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado (proxy 403 em CONNECT) |
| **submd**<br>https://sub.md | GET api.sub.md/v1/search?apex={d} (linhas de texto) | Chave opcional (Bearer); anônimo: 50 consultas/dia, 1 req/s. `LUNATIC_SUBMD_API_KEY` | implementada padrão | Exatamente 1 requisição por execução; sem paginação | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado (proxy 403 em CONNECT) |
| **thc**<br>https://ip.thc.org | POST ip.thc.org/api/v1/lookup/subdomains {domain,page_state,limit:1000} (cursor next_page_state) | Nenhuma; limites/ToS não confirmados | implementada padrão | Página de docs ilegível; campos vêm do Subfinder; parser tolerante; serviço novo | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado (proxy 403 em CONNECT) |
| **threatbook**<br>https://threatbook.io | GET api.threatbook.cn/v3/domain/sub_domains?apikey=&resource={d} | Obrigatória (query `apikey`); cota gratuita para este endpoint não confirmada. `LUNATIC_THREATBOOK_API_KEY` | implementada não-padrão (requer chave) | Endpoint ausente da doc pública internacional (só Subfinder); host .cn; `response_code` mapeado por texto | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **threatcrowd**<br>https://www.threatcrowd.org | Nenhum (não chama a rede) | n/a | desabilitada | Ver seção de desabilitadas | 2026-09-29 | Info/ErrDisabled ✓ / n/a | Desabilitada (ver seção abaixo) |
| **urlscan**<br>https://urlscan.io/docs/api/ | GET urlscan.io/api/v1/search/?q=domain:{d}&size=100&search_after= (NUNCA /api/v1/scan/) | Chave opcional (cabeçalho API-Key); anônimo permitido com cotas pequenas. `LUNATIC_URLSCAN_API_KEY` | implementada padrão | Cota anônima minúscula (429 possível); só resultados de busca, não abre páginas | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado (proxy 403 em CONNECT) |
| **virustotal**<br>https://docs.virustotal.com/reference/domains-relationships | GET www.virustotal.com/api/v3/domains/{d}/subdomains?limit=40&cursor= | Obrigatória, cabeçalho x-apikey; gratuito: 4 req/min, 500/dia, só uso não comercial. `LUNATIC_VIRUSTOTAL_API_KEY` | implementada com chave | Taxa de 1 req/15 s para chaves gratuitas | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **waybackarchive**<br>https://web.archive.org | GET web.archive.org/cdx/search/cdx?url=*.{d}&output=txt&fl=original&collapse=urlkey&limit=50000&showResumeKey=true (host extraído das URLs; páginas arquivadas nunca abertas) | Nenhuma; limites não documentados | implementada padrão | Respostas grandes/lentas, quedas e 429; limitada por MaxPages; nomes históricos | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado (proxy 403 em CONNECT) |
| **whoisxmlapi**<br>https://subdomains.whoisxmlapi.com/api/documentation/making-requests | GET subdomains.whoisxmlapi.com/api/v2?apiKey=&domainName={d}&outputFormat=JSON&searchAfter= | Obrigatória (query `apiKey`); créditos por requisição, créditos gratuitos únicos. `LUNATIC_WHOISXMLAPI_API_KEY` | implementada com chave | Formato dos registros v2 e local de `nextPageSearchAfter` não confirmados (aceita ambos) | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **windvane**<br>https://windvane.lichoin.com | POST windvane.lichoin.com/trpc.backendhub.public.WindvaneService/ListSubDomain {domain,page_request} | Obrigatória, cabeçalho X-Api-Key. `LUNATIC_WINDVANE_API_KEY` | implementada com chave | Docs ilegíveis; formato vem do Subfinder/theHarvester; código de sucesso 0 presumido | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **zoomeyeapi**<br>https://www.zoomeye.ai | POST api.{host}/v2/search {qbase64: domain="{d}", page, pagesize:1000, fields:domain, subtype:web} | `api_key` obrigatória (cabeçalho API-KEY); `host` opcional (ex.: zoomeye.hk; padrão zoomeye.ai; o formato antigo `host:CHAVE` em api_key ainda é aceito). `LUNATIC_ZOOMEYEAPI_API_KEY`, `LUNATIC_ZOOMEYEAPI_HOST` (opcional) | implementada não-padrão (requer chave) | Código de sucesso 60000 presumido; créditos por página; poucos créditos gratuitos mensais | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado; sem chave |
| **subdomaincenter (extra)**<br>https://www.subdomain.center | GET api.subdomain.center/?domain={d} (array JSON) | Nenhuma; gratuito limitado a ~500 resultados aleatórios (segundo o site) | implementada padrão | Teto de resultados; mecanismo de chave paga não confirmado e não usado | 2026-09-29 | fixture ✓ / ao vivo ✗ | egress bloqueado (proxy 403 em CONNECT) |
| **binaryedge (extra)**<br>https://www.binaryedge.io | Nenhum (não chama a rede) | n/a | desabilitada | Ver seção de desabilitadas | 2026-09-29 | Info/ErrDisabled ✓ / n/a | Desabilitada (ver seção abaixo) |

Todas as linhas "implementada ..." usam apenas endpoints de consulta/busca do provedor; nenhuma dispara scans, resolve DNS dos resultados nem contata hosts descobertos.

Contagem de linhas na tabela: 54 fontes registradas (52 da lista pedida + 2 extras).

## Fontes desabilitadas (motivos concretos)

Todas retornam `ErrDisabled` sem tocar a rede e aparecem em `--list-sources` com status `disabled`. Os motivos abaixo são os `DisabledReason` do código.

- **binaryedge** (extra): o serviço e a API foram encerrados em 2025-03-31 (clientes migraram para Coalition Control). Mantida só para a matriz de cobertura ficar completa.
- **censys**: o esquema de busca de certificados da Censys Platform API não está confirmado na documentação oficial (só em código de terceiros); contas gratuitas não podem usar endpoints de busca (medidos por créditos); a API legada com id:secret foi aposentada. Campos reservados: `api_token`, `organization_id`.
- **chinaz**: a API documentada retorna dados Alexa/rank; listagem de subdomínios não confirmada na documentação oficial atual.
- **domainsproject**: API não documentada, disponível só por aprovação; sem documentação pública nem endpoint estável (o endpoint testado pelos pesquisadores respondia 502).
- **hudsonrock**: o conjunto de dados deriva de infecções por infostealers (telemetria de credenciais roubadas). O Lunatic não processa dados derivados de infostealers, por razões éticas e legais; termos de uso também não verificados.
- **robtex**: a API gratuita (10 req/hora) devolve registros de DNS passivo pertencentes ao nome consultado, não uma lista de subdomínios; descobrir subdomínios exigiria a API Pro paga mais uma consulta reversa por IP.
- **threatcrowd**: sem manutenção. A página da API diz que pode ser retirada e "provavelmente instável" (1 req/10 s; clientes conhecidos usam host HTTP simples); os dados são redundantes com o AlienVault OTX. Nenhum aviso formal de encerramento foi encontrado (o host não estava acessível para verificar).

## Extras avaliados e não adicionados

| Fonte | Motivo |
|---|---|
| columbus (columbus.elmasy.com) | Caminho `/api/lookup/{d}`, formato da saída (FQDN ou rótulos) e limites não confirmados; host inacessível e docs ilegíveis. Reavaliar após teste ao vivo. |
| arquivo.pt (CDX) | Arquivo web português; cobertura fraca de domínios globais; não testado ao vivo; baixo valor. |
| Mnemonic (passive DNS) | Acesso público tem limites de recurso e ToS; especificação de consulta por subdomínio/curinga e limites não confirmados. |
| Validin | Exige chave; docs redirecionadas/não lidas; cota gratuita não confirmada. |
| CIRCL (pDNS) | Exige aprovação de conta; não pesquisado a fundo. |
| archive.today | Sem API pública e hostil a acesso automatizado; ignorada por questões de ToS. |

## Endpoints a validar na primeira execução ao vivo

Nomes de campos, caminhos ou cabeçalhos abaixo se apoiam **apenas** no Subfinder, em código de terceiros ou em documentação parcial. Confirme-os na primeira execução real (`scripts/live-smoke.sh`) e corrija o adaptador se divergirem.

- **alienvault**: cabeçalho `X-OTX-API-KEY` (Subfinder usa Bearer; conflito não resolvido).
- **bevigil**, **digitalyama**, **rsecloud**, **quake**, **profundis**, **windvane**, **threatbook**: caminho e corpo vêm do Subfinder (portais de docs ilegíveis ou endpoint ausente da doc pública).
- **bufferover**: formato das strings de resultado (hoje extraído por regex).
- **builtwith**: versão `v26` (doc) vs `v21` (Subfinder); chave na query.
- **c99**: se o endpoint devolve só dados armazenados.
- **dnsrepo**: esquema da resposta (array de `{domain}`) de código de terceiros.
- **driftnet**: valores de `summary_context` por endpoint.
- **fofa**: necessidade do parâmetro `email` em contas antigas; heurística de mensagens de erro.
- **intelx**: chave enviada como `k` (query) e `x-key` (cabeçalho); host da conta.
- **netlas**: corpo do POST `/api/domains/download/`.
- **onyphe**: categoria `resolver` da busca v2.
- **redhuntlabs**: prefixo `community` (outros planos usam outro).
- **scanmalware**: nomes de campos do JSON (parser genérico); cabeçalho de chave opcional não usado.
- **thc**: campos do corpo/resposta; docs ilegíveis.
- **whoisxmlapi**: forma dos registros v2 e posição de `nextPageSearchAfter`.
- **zoomeyeapi**: código de sucesso 60000 e corpo da busca.
- **shodan**, **shodanct**, **commoncrawl** (`showNumPages`), **anubis** (comportamento para domínio desconhecido), **dnsdumpster** (paginação), **hackertarget** (cota exata): detalhes de comportamento não confirmados.
- **digitorus**, **rapiddns**, **sitedossier**: raspagem de HTML; o layout pode mudar a qualquer momento.

## Divergências entre os arquivos de cobertura e o código

Onde os relatórios de pesquisa diferiam do código, esta página segue o código:

- **intelx**: o relatório dizia que `host` não podia ser opcional; o código agora o declara como campo opcional (`OptCredFields`), padrão `free.intelx.io`.
- **zoomeyeapi**: o relatório dobrava o host dentro de `api_key` (`host:CHAVE`); o código tem o campo opcional `host` e mantém o formato antigo por compatibilidade.
- **fofa**: o relatório dizia que `email` não era enviado; o código o envia somente se o campo opcional `email` estiver configurado.
- Demais linhas: nenhuma divergência encontrada entre os relatórios e o código (Auth, Default, Disabled, campos).

## Resumo

| Situação | Quantidade |
|---|---|
| implementada padrão (roda sem configuração) | 12 |
| implementada não-padrão, sem chave (`-s` ou `--all`) | 4 (commoncrawl, digitorus, rapiddns, sitedossier) |
| implementada com chave (padrão só com chave configurada) | 29 |
| implementada não-padrão com chave | 2 (threatbook, zoomeyeapi) |
| desabilitada | 7 |
| **Total registrado** | **54** |
| Confirmadas ao vivo | **0** |

Dentre as 12 padrão sem chave obrigatória, 5 aceitam chave opcional (alienvault, certspotter, hackertarget, submd, urlscan); scanmalware é anônimo (chave opcional não suportada).
