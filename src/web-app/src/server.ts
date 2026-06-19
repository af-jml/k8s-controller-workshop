// Backend for the Report Queue web app.
//
// Responsibilities:
//   - Serve the static frontend (public/).
//   - List and create ReportRequest custom resources via the Kubernetes API.
//   - Stream live updates to the browser using Server-Sent Events (a Kubernetes "watch").
//   - Stream finished PDFs from MinIO back to the browser.
//
// All Kubernetes access uses the in-cluster ServiceAccount (loadFromCluster). For local
// development outside the cluster, it falls back to your default kubeconfig.

import * as path from 'path';
import express, { Request, Response } from 'express';
import * as k8s from '@kubernetes/client-node';
import * as Minio from 'minio';

const GROUP = 'reports.workshop.io';
const VERSION = 'v1alpha1';
const PLURAL = 'reportrequests';

// Second custom resource: Bucket (storage.workshop.io). The controller provisions these
// into MinIO directly — the "Kubernetes as a cloud API" demo.
const BUCKET_GROUP = 'storage.workshop.io';
const BUCKET_VERSION = 'v1alpha1';
const BUCKET_PLURAL = 'buckets';

const NAMESPACE = process.env.NAMESPACE || 'workshop';
const PORT = parseInt(process.env.PORT || '8080', 10);

const MINIO_ENDPOINT = process.env.MINIO_ENDPOINT || 'minio.workshop.svc.cluster.local';
const MINIO_PORT = parseInt(process.env.MINIO_PORT || '9000', 10);
const MINIO_ACCESS_KEY = process.env.MINIO_ACCESS_KEY || 'minioadmin';
const MINIO_SECRET_KEY = process.env.MINIO_SECRET_KEY || 'minioadmin';
const MINIO_BUCKET = process.env.MINIO_BUCKET || 'reports';

// --- Kubernetes client ------------------------------------------------------------------
const kc = new k8s.KubeConfig();
try {
  kc.loadFromCluster();
  console.log('Loaded in-cluster Kubernetes config');
} catch {
  kc.loadFromDefault();
  console.log('Loaded default (kubeconfig) Kubernetes config');
}
const customApi = kc.makeApiClient(k8s.CustomObjectsApi);

// --- MinIO client -----------------------------------------------------------------------
const minioClient = new Minio.Client({
  endPoint: MINIO_ENDPOINT,
  port: MINIO_PORT,
  useSSL: false,
  accessKey: MINIO_ACCESS_KEY,
  secretKey: MINIO_SECRET_KEY,
});

// --- Express app ------------------------------------------------------------------------
const app = express();
app.use(express.json());
app.use(express.static(path.join(__dirname, '..', 'public')));

// Liveness probe.
app.get('/healthz', (_req, res) => res.send('ok'));

// List all ReportRequests.
app.get('/api/reports', async (_req: Request, res: Response) => {
  try {
    const result: any = await customApi.listNamespacedCustomObject(
      GROUP, VERSION, NAMESPACE, PLURAL,
    );
    const items = (result.body?.items || []).map(toView);
    res.json(items);
  } catch (err: any) {
    console.error('list failed', err?.body || err);
    res.status(500).json({ error: 'failed to list reports' });
  }
});

// Create a new ReportRequest.
app.post('/api/reports', async (req: Request, res: Response) => {
  const { title, dataset, instructions } = req.body || {};
  if (!title || !dataset) {
    return res.status(400).json({ error: 'title and dataset are required' });
  }
  const name = `report-${Date.now()}`;
  const body = {
    apiVersion: `${GROUP}/${VERSION}`,
    kind: 'ReportRequest',
    metadata: { name },
    spec: { title, dataset, instructions: instructions || '', format: 'pdf' },
  };
  try {
    const result: any = await customApi.createNamespacedCustomObject(
      GROUP, VERSION, NAMESPACE, PLURAL, body,
    );
    res.status(201).json(toView(result.body));
  } catch (err: any) {
    console.error('create failed', err?.body || err);
    res.status(500).json({ error: 'failed to create report' });
  }
});

// Stream live updates of ReportRequests to the browser via Server-Sent Events.
// This is a Kubernetes "watch": the API server pushes every add/update/delete to us.
app.get('/api/reports/stream', (req: Request, res: Response) => {
  res.set({
    'Content-Type': 'text/event-stream',
    'Cache-Control': 'no-cache',
    Connection: 'keep-alive',
  });
  res.flushHeaders();

  const watch = new k8s.Watch(kc);
  let aborted = false;
  let reqController: { abort: () => void } | undefined;

  const send = (event: string, data: unknown) => {
    res.write(`event: ${event}\n`);
    res.write(`data: ${JSON.stringify(data)}\n\n`);
  };

  const startWatch = async () => {
    try {
      reqController = await watch.watch(
        `/apis/${GROUP}/${VERSION}/namespaces/${NAMESPACE}/${PLURAL}`,
        {},
        (type, obj: any) => send('change', { type, object: toView(obj) }),
        (err) => {
          if (aborted) return;
          if (err) console.error('watch error, restarting', err);
          setTimeout(startWatch, 1000);
        },
      );
    } catch (err) {
      if (!aborted) {
        console.error('watch start failed, retrying', err);
        setTimeout(startWatch, 1000);
      }
    }
  };
  startWatch();

  // Heartbeat so proxies don't close the idle connection.
  const heartbeat = setInterval(() => res.write(': ping\n\n'), 15000);

  req.on('close', () => {
    aborted = true;
    clearInterval(heartbeat);
    reqController?.abort();
  });
});

// Stream a finished PDF from MinIO back to the browser.
app.get('/api/reports/:name/pdf', async (req: Request, res: Response) => {
  const name = req.params.name;
  try {
    const result: any = await customApi.getNamespacedCustomObject(
      GROUP, VERSION, NAMESPACE, PLURAL, name,
    );
    const objectKey: string | undefined = result.body?.status?.pdfObjectKey;
    if (!objectKey) {
      return res.status(404).json({ error: 'PDF not ready' });
    }
    res.set({
      'Content-Type': 'application/pdf',
      'Content-Disposition': `inline; filename="${name}.pdf"`,
    });
    const stream = await minioClient.getObject(MINIO_BUCKET, objectKey);
    stream.on('error', (err) => {
      console.error('minio stream error', err);
      if (!res.headersSent) res.status(500).end();
    });
    stream.pipe(res);
  } catch (err: any) {
    console.error('pdf fetch failed', err?.body || err);
    res.status(500).json({ error: 'failed to fetch PDF' });
  }
});

// --- Bucket endpoints (storage.workshop.io) ---------------------------------------------

// List all Buckets.
app.get('/api/buckets', async (_req: Request, res: Response) => {
  try {
    const result: any = await customApi.listNamespacedCustomObject(
      BUCKET_GROUP, BUCKET_VERSION, NAMESPACE, BUCKET_PLURAL,
    );
    const items = (result.body?.items || []).map(toBucketView);
    res.json(items);
  } catch (err: any) {
    console.error('list buckets failed', err?.body || err);
    res.status(500).json({ error: 'failed to list buckets' });
  }
});

// Create a new Bucket. The web app can create the custom resource, but only the controller
// can provision the real bucket in MinIO — same RBAC boundary as ReportRequest.
app.post('/api/buckets', async (req: Request, res: Response) => {
  const { bucketName, accessPolicy, quota, deletionPolicy } = req.body || {};
  if (!bucketName) {
    return res.status(400).json({ error: 'bucketName is required' });
  }
  if (!/^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$/.test(bucketName)) {
    return res.status(400).json({
      error: 'bucketName must be 3-63 chars: lowercase letters, numbers, dots and hyphens',
    });
  }
  const body = {
    apiVersion: `${BUCKET_GROUP}/${BUCKET_VERSION}`,
    kind: 'Bucket',
    // Use the bucket name as the object name: a clean 1:1 mapping (and a natural
    // uniqueness check — a duplicate returns 409).
    metadata: { name: bucketName },
    spec: {
      bucketName,
      accessPolicy: accessPolicy === 'public-read' ? 'public-read' : 'private',
      quota: quota || '',
      deletionPolicy: deletionPolicy === 'Delete' ? 'Delete' : 'Retain',
    },
  };
  try {
    const result: any = await customApi.createNamespacedCustomObject(
      BUCKET_GROUP, BUCKET_VERSION, NAMESPACE, BUCKET_PLURAL, body,
    );
    res.status(201).json(toBucketView(result.body));
  } catch (err: any) {
    if (err?.body?.code === 409 || err?.statusCode === 409) {
      return res.status(409).json({ error: `bucket "${bucketName}" already exists` });
    }
    console.error('create bucket failed', err?.body || err);
    res.status(500).json({ error: 'failed to create bucket' });
  }
});

// Stream live Bucket updates to the browser via Server-Sent Events (a Kubernetes watch).
app.get('/api/buckets/stream', (req: Request, res: Response) => {
  res.set({
    'Content-Type': 'text/event-stream',
    'Cache-Control': 'no-cache',
    Connection: 'keep-alive',
  });
  res.flushHeaders();

  const watch = new k8s.Watch(kc);
  let aborted = false;
  let reqController: { abort: () => void } | undefined;

  const send = (event: string, data: unknown) => {
    res.write(`event: ${event}\n`);
    res.write(`data: ${JSON.stringify(data)}\n\n`);
  };

  const startWatch = async () => {
    try {
      reqController = await watch.watch(
        `/apis/${BUCKET_GROUP}/${BUCKET_VERSION}/namespaces/${NAMESPACE}/${BUCKET_PLURAL}`,
        {},
        (type, obj: any) => send('change', { type, object: toBucketView(obj) }),
        (err) => {
          if (aborted) return;
          if (err) console.error('bucket watch error, restarting', err);
          setTimeout(startWatch, 1000);
        },
      );
    } catch (err) {
      if (!aborted) {
        console.error('bucket watch start failed, retrying', err);
        setTimeout(startWatch, 1000);
      }
    }
  };
  startWatch();

  const heartbeat = setInterval(() => res.write(': ping\n\n'), 15000);

  req.on('close', () => {
    aborted = true;
    clearInterval(heartbeat);
    reqController?.abort();
  });
});

// Map a raw Bucket object to the shape the frontend needs.
function toBucketView(obj: any) {
  return {
    name: obj?.metadata?.name,
    creationTimestamp: obj?.metadata?.creationTimestamp,
    bucketName: obj?.spec?.bucketName,
    accessPolicy: obj?.spec?.accessPolicy || 'private',
    quota: obj?.spec?.quota || '',
    deletionPolicy: obj?.spec?.deletionPolicy || 'Retain',
    phase: obj?.status?.phase || 'Pending',
    message: obj?.status?.message || '',
    endpoint: obj?.status?.endpoint || '',
  };
}

// Map a raw ReportRequest object to the shape the frontend needs.
function toView(obj: any) {
  return {
    name: obj?.metadata?.name,
    creationTimestamp: obj?.metadata?.creationTimestamp,
    title: obj?.spec?.title,
    dataset: obj?.spec?.dataset,
    instructions: obj?.spec?.instructions,
    phase: obj?.status?.phase || 'Pending',
    message: obj?.status?.message || '',
    jobName: obj?.status?.jobName || '',
    pdfObjectKey: obj?.status?.pdfObjectKey || '',
  };
}

app.listen(PORT, () => {
  console.log(`Report Queue web app listening on :${PORT} (namespace=${NAMESPACE})`);
});
