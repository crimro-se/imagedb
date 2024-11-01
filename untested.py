from flask import Flask, request, jsonify
from multiprocessing import Queue, Process, Manager
from queue import Empty
import base64
import time
import uuid
import torch
import multiprocessing as mp
from transformers import CLIPProcessor, CLIPModel
from PIL import Image
import io

BATCH_SIZE = 10  # Define the batch size




def process_tasks(task_queue, results):
    device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
    model = CLIPModel.from_pretrained("openai/clip-vit-base-patch32").to(device)
    processor = CLIPProcessor.from_pretrained("openai/clip-vit-base-patch32")

    while True:
        batch = []
        # we block to get a task to avoid busy looping...
        task = task_queue.get()
        if task is None:
            break # Stop signal
        batch.append(task)
        # now we fill up the batch as much as possible
        if not get_additional_tasks(batch, task_queue):
            break

        img_batch, txt_batch = [],[]
        images, texts = [],[]
        print (len(batch))
        for t in batch:
            task_id, image_data, text_data = t
            if image_data:
                img_batch.append((task_id, base64.b64decode(image_data)))
            if text_data:
                txt_batch.append((task_id, text_data))

        if img_batch:
            images = [Image.open(io.BytesIO(data[1])) for data in img_batch]
            inputs = processor(images=images, return_tensors="pt").to(device)
            with torch.no_grad():
                image_embeddings = model.get_image_features(**inputs).cpu().numpy().tolist()
            for i, emb in enumerate(image_embeddings):
                results[img_batch[i][0]] = image_embeddings[i]
        
        if txt_batch:
            texts = [t[1] for t in txt_batch]
            inputs = processor(text=texts, return_tensors="pt").to(device)
            with torch.no_grad():
                text_embeddings = model.get_text_features(**inputs).cpu().numpy().tolist()
                for i, emb in enumerate(text_embeddings):
                    results[txt_batch[i][0]] = emb

def get_additional_tasks(batch, q):
    try: 
        while len(batch) < BATCH_SIZE:
            task = q.get_nowait()
            if task is None: 
                return False
            batch.append(task)
    except Empty:
        return True
    return True

if __name__ == '__main__':
    mp.set_start_method('spawn')
    # Start the task processor in the background

    app = Flask(__name__)
    # Shared Queue for Tasks
    task_queue = Queue()

    # Shared Dictionary for results
    mgr = Manager() 
    results = mgr.dict()

    processor_process = Process(target=process_tasks, args=(task_queue,results))
    processor_process.daemon = True  # So it exits when main process exits
    processor_process.start()

    @app.route('/process', methods=['POST'])
    def process_endpoint():
        data = request.get_json()
        if not data:
            return jsonify({'error': 'No JSON data received'}), 400
        
        task_id = data.get('id') or str(uuid.uuid4())
        image_data = data.get('image')
        text_data = data.get('text')
        
        if not (image_data or text_data):
            return jsonify({'error': 'Either image or text data is required'}), 400
        
        task_queue.put((task_id, image_data, text_data))
        return jsonify({'message': 'Task submitted successfully', 'id': task_id}), 202

    @app.route('/result/<string:id>', methods=['GET'])
    def result_endpoint(id):
        if id not in results:
            return jsonify({'error': 'Result not found or not yet processed'}), 404
        return jsonify({'id': id, 'result': results[id]}), 200
    
    try:
        app.run(debug=False)
    finally:
        task_queue.put(None)  # Stop signal
        processor_process.join()