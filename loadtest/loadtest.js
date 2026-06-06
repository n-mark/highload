import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = 'http://localhost:8080';
const USERS_COUNT = parseInt(__ENV.USERS || '50');

/**
 * Генерация простой профилировочной даты (без сложной логики)
 */
function generateProfileData() {
  return {
    name: `u_${Math.random().toString(36).substring(2, 8)}`,
    surname: `s_${Math.random().toString(36).substring(2, 8)}`,
    date_of_birth: '1990-01-01T00:00:00.000Z',
    gender: 'MALE',
    interests: 'test',
    city: 'test',
  };
}

/**
 * LOAD CONFIG
 */
export const options = {
  scenarios: {
    send_load: {
      executor: 'constant-arrival-rate',
      rate: 200,
      timeUnit: '1s',
      duration: '3m',
      preAllocatedVUs: 100,
      maxVUs: 300,
      exec: 'sendScenario',
    },

    list_load: {
      executor: 'constant-arrival-rate',
      rate: 100,
      timeUnit: '1s',
      duration: '3m',
      preAllocatedVUs: 50,
      maxVUs: 150,
      exec: 'listScenario',
    },
  },

  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<200'],
  },
};

/**
 * SETUP: создаём пользователей + профили
 */
export function setup() {
  console.log(`Creating ${USERS_COUNT} users...`);

  const users = [];

  for (let i = 0; i < USERS_COUNT; i++) {
    const username = `u_${i}_${Date.now()}`;
    const email = `${username}@test.com`;
    const password = 'Qwerty123!';

    // 1. REGISTER
    const reg = http.post(
      `${BASE_URL}/auth/register`,
      JSON.stringify({ username, email, password }),
      { headers: { 'Content-Type': 'application/json' } }
    );

    if (![200, 201].includes(reg.status)) {
      console.error('register failed', reg.status, reg.body);
      continue;
    }

    // 2. LOGIN
    const loginRes = http.post(
      `${BASE_URL}/auth/login`,
      JSON.stringify({ username, password }),
      { headers: { 'Content-Type': 'application/json' } }
    );

    if (loginRes.status !== 200) {
      console.error('login failed', loginRes.status, loginRes.body);
      continue;
    }

    const login = JSON.parse(loginRes.body);
    const token = login.access_token;

    // 3. CREATE PROFILE (ВАЖНО!)
    const profileRes = http.post(
      `${BASE_URL}/profile`,
      JSON.stringify(generateProfileData()),
      {
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (![200, 201].includes(profileRes.status)) {
      console.error('profile create failed', profileRes.status, profileRes.body);
      continue;
    }

    const profile = JSON.parse(profileRes.body);

    const id =
      profile.user_id ||
      profile.id ||
      login.profile_id;

    if (!id) {
      throw new Error('NO PROFILE ID RETURNED');
    }

    users.push({
      id,
      token,
    });

    console.log(`[${i + 1}/${USERS_COUNT}] user ready id=${id}`);
  }

  if (users.length < 2) {
    throw new Error('Not enough users created');
  }

  return { users };
}

/**
 * helper: pair generator
 */
function getPair(users) {
  const me = users[Math.floor(Math.random() * users.length)];

  let peer = users[Math.floor(Math.random() * users.length)];
  while (peer.id === me.id) {
    peer = users[Math.floor(Math.random() * users.length)];
  }

  return { me, peer };
}

/**
 * SEND MESSAGE → Tarantool write path
 */
export function sendScenario(data) {
  const { me, peer } = getPair(data.users);

  const res = http.post(
    `${BASE_URL}/dialog/${peer.id}/send`,
    JSON.stringify({
      text: `msg_${Date.now()}`,
    }),
    {
      headers: {
        Authorization: `Bearer ${me.token}`,
        'Content-Type': 'application/json',
      },
    }
  );

  check(res, {
    'send ok': (r) => r.status === 201,
  });
}

/**
 * LIST MESSAGES → Tarantool read path
 */
export function listScenario(data) {
  const { me, peer } = getPair(data.users);

  const res = http.get(
    `${BASE_URL}/dialog/${peer.id}/list`,
    {
      headers: {
        Authorization: `Bearer ${me.token}`,
      },
    }
  );

  check(res, {
    'list ok': (r) => r.status === 200,
  });
}