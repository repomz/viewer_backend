-- +goose Up
-- +goose StatementBegin
INSERT INTO studies (
    id,
    created_at,
    updated_at,
    study_id,
    patient,
    age,
    department,
    name_operation,
    descr_operation,
    time_beginning,
    time_duration,
    surgeon,
    dicom_link
) VALUES
    (
        DEFAULT,                                  -- id: автоматический UUID
        '2025-03-10 10:00:00+03',                 -- created_at
        '2025-03-10 10:00:00+03',                 -- updated_at
        '988',                                    -- studyID
        'Пантелеев И.И.',                         -- patient
        69,                                       -- age
        'кардио №2',                              -- department
        'БАП стент стЛКА в ОА',                   -- nameOperation
        'что то там очень важное',                -- descrOperation
        '2025-03-10 13:00:00+03',                 -- timeBeginning
        40,                                       -- timeDuration (минуты)
        'Старков А.С.',                           -- surgeon
        ''                                        -- dicomLink (пустая строка)
    ),
    (
        DEFAULT,
        '2025-03-11 10:00:00+03',
        '2025-03-11 10:00:00+03',
        '1001',
        'Сидорова М.А.',
        58,
        'нейрохирургия',
        'Удаление менингиомы',
        'левая лобная доля',
        '2025-03-11 09:30:00+03',
        120,
        'Иванов В.П.',
        'https://example.com/dicom/1001'
    ),
    (
        DEFAULT,
        '2025-03-12 10:00:00+03',
        '2025-03-12 10:00:00+03',
        '1002',
        'Козлов Д.С.',
        45,
        'травматология',
        'Остеосинтез бедра',
        'интрамедуллярный стержень',
        '2025-03-12 11:15:00+03',
        90,
        'Петров С.Н.',
        ''
    ),
    (
        DEFAULT,
        '2025-03-13 10:00:00+03',
        '2025-03-13 10:00:00+03',
        '1003',
        'Морозова Е.В.',
        55,                                     -- возраст не указан
        'кардио №1',
        'АКШ',
        'аортокоронарное шунтирование 3 сосудов',
        '2025-03-13 08:00:00+03',
        240,
        'Воронина Л.К.',
        '/dicom/studies/1003'
    ),
    (
        DEFAULT,
        '2025-03-14 10:00:00+03',
        '2025-03-14 10:00:00+03',
        '1004',
        'Лебедев А.А.',
        72,
        'урология',
        'ТУР простаты',
        'трансуретральная резекция',
        '2025-03-14 14:45:00+03',
        55,
        'Медведев О.И.',
        ''
    );
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
TRUNCATE TABLE studies RESTART IDENTITY;
-- +goose StatementEnd
