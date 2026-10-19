-- отключаем права на удаление финансовых записей
REVOKE DELETE ON 
    subscription, 
    payment_order, 
    payment_attempt,
    provider_webhook_event, 
    refund, 
    chargeback, 
    subscription_period, 
    donation 
FROM app_user;