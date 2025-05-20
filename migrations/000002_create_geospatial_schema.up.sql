
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS timescaledb;
CREATE EXTENSION IF NOT EXISTS h3; -- Для гексагональної індексації

CREATE TABLE IF NOT EXISTS sensor_data (
                                           id UUID PRIMARY KEY,
                                           scan_id UUID NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
    sensor_type TEXT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    location GEOGRAPHY(POINT, 4326),
    altitude FLOAT,
    data JSONB,
    quality_indicators JSONB
    );


SELECT create_hypertable('sensor_data', 'timestamp',
                         chunk_time_interval => INTERVAL '1 hour',
                         if_not_exists => TRUE);


CREATE INDEX IF NOT EXISTS sensor_data_location_idx
    ON sensor_data USING GIST (location);


CREATE INDEX IF NOT EXISTS sensor_data_type_idx
    ON sensor_data (sensor_type);


CREATE INDEX IF NOT EXISTS sensor_data_scan_time_idx
    ON sensor_data (scan_id, timestamp);


CREATE TABLE IF NOT EXISTS raw_data_files (
                                              id UUID PRIMARY KEY,
                                              scan_id UUID NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
    sensor_type TEXT NOT NULL,
    object_key TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    content_type TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

CREATE INDEX IF NOT EXISTS raw_data_files_scan_id_idx ON raw_data_files(scan_id);
CREATE INDEX IF NOT EXISTS raw_data_files_object_key_idx ON raw_data_files(object_key);


CREATE TABLE IF NOT EXISTS heatmap_data (
                                            id UUID PRIMARY KEY,
                                            scan_id UUID NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
    sensor_type TEXT NOT NULL,
    center_lat FLOAT NOT NULL,
    center_lon FLOAT NOT NULL,
    h3_index TEXT NOT NULL,
    point_count INT NOT NULL,
    average_value FLOAT,
    data_json JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

CREATE INDEX IF NOT EXISTS heatmap_data_scan_id_idx ON heatmap_data(scan_id);
CREATE INDEX IF NOT EXISTS heatmap_data_h3_index_idx ON heatmap_data(h3_index);


CREATE TABLE IF NOT EXISTS time_series_aggregations (
                                                        id UUID PRIMARY KEY,
                                                        scan_id UUID NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
    sensor_type TEXT NOT NULL,
    time_bucket TIMESTAMPTZ NOT NULL,
    interval_length TEXT NOT NULL,
    reading_count INT NOT NULL,
    avg_latitude FLOAT,
    avg_longitude FLOAT,
    avg_altitude FLOAT,
    min_value FLOAT,
    max_value FLOAT,
    avg_value FLOAT,
    std_dev FLOAT,
    data_json JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

CREATE INDEX IF NOT EXISTS time_series_scan_id_idx ON time_series_aggregations(scan_id);
CREATE INDEX IF NOT EXISTS time_series_time_bucket_idx ON time_series_aggregations(time_bucket);

CREATE OR REPLACE FUNCTION update_heatmap_aggregation(
    p_scan_id UUID,
    p_sensor_type TEXT,
    p_resolution INT DEFAULT 9
)
RETURNS VOID AS $$
BEGIN

DELETE FROM heatmap_data
WHERE scan_id = p_scan_id AND sensor_type = p_sensor_type;


INSERT INTO heatmap_data (
    id, scan_id, sensor_type, center_lat, center_lon,
    h3_index, point_count, average_value, data_json, created_at
)
SELECT
    gen_random_uuid(),
    p_scan_id,
    p_sensor_type,
    AVG(ST_Y(location::geometry)) AS center_lat,
    AVG(ST_X(location::geometry)) AS center_lon,
    h3_lat_lng_to_cell(ST_Y(location::geometry), ST_X(location::geometry), p_resolution) AS h3_index,
    COUNT(*) AS point_count,
    AVG((data->>'value')::float) AS average_value,
    jsonb_build_object(
            'center_lat', AVG(ST_Y(location::geometry)),
            'center_lon', AVG(ST_X(location::geometry)),
            'point_count', COUNT(*),
            'avg_value', AVG((data->>'value')::float),
            'min_value', MIN((data->>'value')::float),
            'max_value', MAX((data->>'value')::float)
    ) AS data_json,
    NOW()
FROM sensor_data
WHERE
    scan_id = p_scan_id
  AND sensor_type = p_sensor_type
GROUP BY
    h3_lat_lng_to_cell(ST_Y(location::geometry), ST_X(location::geometry), p_resolution);
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION update_time_series_aggregation(
    p_scan_id UUID,
    p_sensor_type TEXT,
    p_interval TEXT DEFAULT '5 minutes'
)
RETURNS VOID AS $$
BEGIN

DELETE FROM time_series_aggregations
WHERE scan_id = p_scan_id
  AND sensor_type = p_sensor_type
  AND interval_length = p_interval;


INSERT INTO time_series_aggregations (
    id, scan_id, sensor_type, time_bucket, interval_length,
    reading_count, avg_latitude, avg_longitude, avg_altitude,
    min_value, max_value, avg_value, std_dev, data_json, created_at
)
SELECT
    gen_random_uuid(),
    p_scan_id,
    p_sensor_type,
    time_bucket(p_interval::interval, timestamp) AS time_bucket,
    p_interval,
    COUNT(*) AS reading_count,
    AVG(ST_Y(location::geometry)) AS avg_latitude,
    AVG(ST_X(location::geometry)) AS avg_longitude,
    AVG(altitude) AS avg_altitude,
    MIN((data->>'value')::float) AS min_value,
    MAX((data->>'value')::float) AS max_value,
    AVG((data->>'value')::float) AS avg_value,
    STDDEV((data->>'value')::float) AS std_dev,
    jsonb_build_object(
            'time_bucket', time_bucket(p_interval::interval, timestamp),
            'reading_count', COUNT(*),
            'avg_latitude', AVG(ST_Y(location::geometry)),
            'avg_longitude', AVG(ST_X(location::geometry)),
            'avg_altitude', AVG(altitude),
            'min_value', MIN((data->>'value')::float),
            'max_value', MAX((data->>'value')::float),
            'avg_value', AVG((data->>'value')::float),
            'std_dev', STDDEV((data->>'value')::float)
    ) AS data_json,
    NOW()
FROM sensor_data
WHERE
    scan_id = p_scan_id
  AND sensor_type = p_sensor_type
GROUP BY
    time_bucket(p_interval::interval, timestamp);
END;
$$ LANGUAGE plpgsql;