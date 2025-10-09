#!/bin/bash

# Script de test de charge pour l'API Analytics
# Teste comment le système performe sous charge

API_URL="http://localhost:8080"
NUM_REQUESTS=${1:-1000}  # Nombre de requêtes (default: 1000)
CONCURRENCY=${2:-10}    # Nombre de requêtes parallèles (default: 10)

echo "🚀 Load Testing Analytics API"
echo "========================================="
echo "API URL: $API_URL"
echo "Total Requests: $NUM_REQUESTS"
echo "Concurrency: $CONCURRENCY"
echo "========================================="
echo ""

# Couleurs
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Fonction pour envoyer un événement
send_event() {
    local event_type=$1
    local user_id=$2
    local amount=$3

    if [ -z "$amount" ]; then
        curl -s -X POST "$API_URL/api/v1/events" \
            -H "Content-Type: application/json" \
            -d "{
                \"type\": \"$event_type\",
                \"user_id\": \"$user_id\",
                \"properties\": {
                    \"page\": \"/page_$(($RANDOM % 100))\"
                }
            }" > /dev/null
    else
        curl -s -X POST "$API_URL/api/v1/events" \
            -H "Content-Type: application/json" \
            -d "{
                \"type\": \"$event_type\",
                \"user_id\": \"$user_id\",
                \"properties\": {
                    \"amount\": $amount
                }
            }" > /dev/null
    fi
}

# Test 1: Envoi de requêtes individuelles
echo -e "${BLUE}TEST 1: Envoi de $NUM_REQUESTS événements individuels${NC}"
echo "Type de distribution: 70% pageview, 20% click, 10% purchase"
echo ""

start_time=$(date +%s%N)
success=0
failed=0

for i in $(seq 1 $NUM_REQUESTS); do
    rand=$((RANDOM % 100))
    user_id="user_$((($RANDOM % 100) + 1))"

    if [ $rand -lt 70 ]; then
        send_event "pageview" "$user_id"
    elif [ $rand -lt 90 ]; then
        send_event "click" "$user_id"
    else
        amount=$((($RANDOM % 500) + 10))
        send_event "purchase" "$user_id" "$amount"
    fi

    ((success++))

    # Afficher la progression tous les 100 requêtes
    if [ $((i % 100)) -eq 0 ]; then
        echo "  ✓ $i/$NUM_REQUESTS requêtes envoyées"
    fi
done

end_time=$(date +%s%N)
duration=$((($end_time - $start_time) / 1000000))  # Convertir en ms

requests_per_second=$(awk -v n="$NUM_REQUESTS" -v d="$duration" 'BEGIN { if (d>0) printf("%.2f", n*1000/d); else print 0 }')

echo ""
echo -e "${GREEN}✅ Test 1 Complété${NC}"
echo "  Temps total: ${duration}ms"
echo "  Requêtes/seconde: $requests_per_second"
echo ""

# Attendre un peu
sleep 2

# Test 2: Batch requests
echo -e "${BLUE}TEST 2: Envoi en batch${NC}"
echo "Envoi de $((NUM_REQUESTS / 10)) batches de 10 événements"
echo ""

start_time=$(date +%s%N)

for batch in $(seq 1 $((NUM_REQUESTS / 10))); do
    # Créer un batch de 10 événements
    batch_json="["
    for i in $(seq 1 10); do
        user_id="batch_user_$((($RANDOM % 50) + 1))"
        event_type=$((($RANDOM % 3) + 1))

        if [ $event_type -eq 1 ]; then
            batch_json="${batch_json}{\"type\":\"pageview\",\"user_id\":\"$user_id\",\"properties\":{\"page\":\"/page_$i\"}},"
        elif [ $event_type -eq 2 ]; then
            batch_json="${batch_json}{\"type\":\"click\",\"user_id\":\"$user_id\",\"properties\":{\"element\":\"button\"}},"
        else
            amount=$((($RANDOM % 500) + 10))
            batch_json="${batch_json}{\"type\":\"purchase\",\"user_id\":\"$user_id\",\"properties\":{\"amount\":$amount}},"
        fi
    done

    # Retirer la dernière virgule et fermer
    batch_json="${batch_json%,}]"

    # Envoyer le batch
    curl -s -X POST "$API_URL/api/v1/events/batch" \
        -H "Content-Type: application/json" \
        -d "$batch_json" > /dev/null

    if [ $((batch % 20)) -eq 0 ]; then
        echo "  ✓ Batch $batch/$((NUM_REQUESTS / 10)) envoyé"
    fi
done

end_time=$(date +%s%N)
duration=$((($end_time - $start_time) / 1000000))

batches=$((NUM_REQUESTS / 10))
batch_per_second=$(awk -v b="$batches" -v d="$duration" 'BEGIN { if (d>0) printf("%.2f", b*1000/d); else print 0 }')

echo ""
echo -e "${GREEN}✅ Test 2 Complété${NC}"
echo "  Temps total: ${duration}ms"
echo "  Batches/seconde: $batch_per_second"
echo ""

# Attendre
sleep 3

# Test 3: Récupérer les métriques
echo -e "${BLUE}TEST 3: Récupération des métriques${NC}"

echo "  Requête 1: GET /api/v1/metrics"
start_time=$(date +%s%N)
response=$(curl -s "$API_URL/api/v1/metrics")
end_time=$(date +%s%N)
latency=$(( ($end_time - $start_time) / 1000000 ))

echo "    ✓ Latence: ${latency}ms"
echo "    ✓ Réponse:"
if command -v jq >/dev/null 2>&1; then
  echo "$response" | jq '
  {
    status: .status,
    metrics_count: (.data | length),
    total_events: (.data.total_events.value // 0),
    pageviews: (.data.pageviews.value // 0),
    clicks: (.data.clicks.value // 0),
    purchases: (.data.purchases.value // 0),
    revenue: (.data.revenue.value // 0),
    unique_users: (.data.unique_users.count // 0)
  }' 2>/dev/null || echo "Error parsing response"
else
  echo "$response"
fi

echo ""

echo "  Requête 2: GET /api/v1/stats"
start_time=$(date +%s%N)
response=$(curl -s "$API_URL/api/v1/stats")
end_time=$(date +%s%N)
latency=$(( ($end_time - $start_time) / 1000000 ))

echo "    ✓ Latence: ${latency}ms"
echo "    ✓ Stats:"
if command -v jq >/dev/null 2>&1; then
  echo "$response" | jq '.data' 2>/dev/null || echo "Error parsing response"
else
  echo "$response"
fi

echo ""

# Test 4: Stress test (requêtes rapides)
echo -e "${BLUE}TEST 4: Stress Test (requêtes aussi rapides que possible)${NC}"
echo "Envoi de 500 requêtes en parallèle avec concurrence $CONCURRENCY"
echo ""

start_time=$(date +%s%N)

# Fonction pour envoyer plusieurs requêtes en parallèle
send_parallel() {
    for i in $(seq 1 500); do
        send_event "pageview" "stress_user_$((i % 50))" &
        
        # Limiter le nombre de processus parallèles
        if [ $((i % CONCURRENCY)) -eq 0 ]; then
            wait
        fi
    done
    wait
}

send_parallel

end_time=$(date +%s%N)
duration=$((($end_time - $start_time) / 1000000))

stress_rps=$(awk -v n=500 -v d="$duration" 'BEGIN { if (d>0) printf("%.2f", n*1000/d); else print 0 }')

echo ""
echo -e "${GREEN}✅ Test 4 Complété${NC}"
echo "  Temps total: ${duration}ms"
echo "  Requêtes/seconde (pic): $stress_rps"
echo ""

# Résumé final
echo "========================================="
echo -e "${GREEN}📊 RÉSUMÉ DES TESTS${NC}"
echo "========================================="
echo ""

echo "Récupérer les métriques finales..."
sleep 1

final_metrics=$(curl -s "$API_URL/api/v1/metrics")

if command -v jq >/dev/null 2>&1; then
  echo "$final_metrics" | jq '
  .data as $metrics |
  {
    total_events: $metrics.total_events.value,
    pageviews: $metrics.pageviews.value,
    clicks: $metrics.clicks.value,
    purchases: $metrics.purchases.value,
    revenue: $metrics.revenue.value,
    unique_users: $metrics.unique_users.count,
    unique_pages: $metrics.unique_pages.count,
    unique_sessions: $metrics.unique_sessions.count
  }' 2>/dev/null
else
  echo "$final_metrics"
fi

echo ""
echo -e "${GREEN}✅ Tous les tests complétés !${NC}"
echo ""