/*
Copyright 2023 Chia Network Inc.
*/

package chiawallet

import (
	"context"
	"fmt"
	"strings"
	"time"

	k8schianetv1 "github.com/chia-network/chia-operator/api/v1"
	"github.com/chia-network/chia-operator/internal/controller/common/kube"
	"github.com/chia-network/chia-operator/internal/metrics"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ChiaWalletReconciler reconciles a ChiaWallet object
type ChiaWalletReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

var chiawallets = make(map[string]bool)

//+kubebuilder:rbac:groups=k8s.chia.net,resources=chiawallets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.chia.net,resources=chiawallets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.chia.net,resources=chiawallets/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is invoked on any event to a controlled Kubernetes resource
func (r *ChiaWalletReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Running reconciler...")

	// Get the custom resource
	var wallet k8schianetv1.ChiaWallet
	err := r.Get(ctx, req.NamespacedName, &wallet)
	if err != nil && errors.IsNotFound(err) {
		// Remove this object from the map for tracking and subtract this CR's total metric by 1
		_, exists := chiawallets[req.String()]
		if exists {
			delete(chiawallets, req.String())
			metrics.ChiaWallets.Sub(1.0)
		}
		return ctrl.Result{}, nil
	}
	if err != nil {
		log.Error(err, fmt.Sprintf("ChiaWalletReconciler ChiaWallet=%s unable to fetch ChiaWallet resource", req.NamespacedName))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Add this object to the tracking map and increment the gauge by 1, if it wasn't already added
	_, exists := chiawallets[req.String()]
	if !exists {
		chiawallets[req.String()] = true
		metrics.ChiaWallets.Add(1.0)
	}

	// Check for ChiaNetwork, retrieve matching ConfigMap if specified
	networkData, err := kube.GetChiaNetworkData(ctx, r.Client, wallet.Spec.ChiaConfig.CommonSpecChia, wallet.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Assemble Peer Service
	peerSrv := assemblePeerService(wallet)
	if err := controllerutil.SetControllerReference(&wallet, &peerSrv, r.Scheme); err != nil {
		r.Recorder.Event(&wallet, corev1.EventTypeWarning, "Failed", "Failed to assemble wallet peer Service -- Check operator logs.")
		return ctrl.Result{}, fmt.Errorf("ChiaWalletReconciler ChiaWallet=%s encountered error assembling peer Service: %v", req.NamespacedName, err)
	}
	// Reconcile Peer Service
	res, err := kube.ReconcileService(ctx, r.Client, wallet.Spec.ChiaConfig.PeerService, peerSrv, true)
	if err != nil {
		return res, fmt.Errorf("ChiaWalletReconciler ChiaWallet=%s %v", req.NamespacedName, err)
	}

	// Assemble All Service
	allSrv := assembleAllService(wallet)
	if err := controllerutil.SetControllerReference(&wallet, &allSrv, r.Scheme); err != nil {
		r.Recorder.Event(&wallet, corev1.EventTypeWarning, "Failed", "Failed to assemble wallet all-port Service -- Check operator logs.")
		return ctrl.Result{}, fmt.Errorf("ChiaWalletReconciler ChiaWallet=%s encountered error assembling all-port Service: %v", req.NamespacedName, err)
	}
	// Reconcile All Service
	res, err = kube.ReconcileService(ctx, r.Client, wallet.Spec.ChiaConfig.AllService, allSrv, true)
	if err != nil {
		return res, fmt.Errorf("ChiaWalletReconciler ChiaWallet=%s %v", req.NamespacedName, err)
	}

	// Assemble Daemon Service
	daemonSrv := assembleDaemonService(wallet)
	if err := controllerutil.SetControllerReference(&wallet, &daemonSrv, r.Scheme); err != nil {
		r.Recorder.Event(&wallet, corev1.EventTypeWarning, "Failed", "Failed to assemble wallet daemon Service -- Check operator logs.")
		return ctrl.Result{}, fmt.Errorf("ChiaWalletReconciler ChiaWallet=%s encountered error assembling daemon Service: %v", req.NamespacedName, err)
	}
	// Reconcile Daemon Service
	res, err = kube.ReconcileService(ctx, r.Client, wallet.Spec.ChiaConfig.DaemonService, daemonSrv, true)
	if err != nil {
		return res, fmt.Errorf("ChiaWalletReconciler ChiaWallet=%s %v", req.NamespacedName, err)
	}

	// Assemble RPC Service
	rpcSrv := assembleRPCService(wallet)
	if err := controllerutil.SetControllerReference(&wallet, &rpcSrv, r.Scheme); err != nil {
		r.Recorder.Event(&wallet, corev1.EventTypeWarning, "Failed", "Failed to assemble wallet RPC Service -- Check operator logs.")
		return ctrl.Result{}, fmt.Errorf("ChiaWalletReconciler ChiaWallet=%s encountered error assembling RPC Service: %v", req.NamespacedName, err)
	}
	// Reconcile RPC Service
	res, err = kube.ReconcileService(ctx, r.Client, wallet.Spec.ChiaConfig.RPCService, rpcSrv, true)
	if err != nil {
		return res, fmt.Errorf("ChiaWalletReconciler ChiaWallet=%s %v", req.NamespacedName, err)
	}

	// Assemble Chia-Exporter Service
	exporterSrv := assembleChiaExporterService(wallet)
	if err := controllerutil.SetControllerReference(&wallet, &exporterSrv, r.Scheme); err != nil {
		r.Recorder.Event(&wallet, corev1.EventTypeWarning, "Failed", "Failed to assemble wallet chia-exporter Service -- Check operator logs.")
		return ctrl.Result{}, fmt.Errorf("ChiaWalletReconciler ChiaWallet=%s encountered error assembling chia-exporter Service: %v", req.NamespacedName, err)
	}
	// Reconcile Chia-Exporter Service
	res, err = kube.ReconcileService(ctx, r.Client, wallet.Spec.ChiaExporterConfig.Service, exporterSrv, true)
	if err != nil {
		return res, fmt.Errorf("ChiaWalletReconciler ChiaWallet=%s %v", req.NamespacedName, err)
	}

	// Creates a persistent volume claim if the GenerateVolumeClaims setting was set to true
	if kube.ShouldMakeChiaRootVolumeClaim(wallet.Spec.Storage) {
		pvc, err := assembleVolumeClaim(wallet)
		if err != nil {
			r.Recorder.Event(&wallet, corev1.EventTypeWarning, "Failed", "Failed to assemble wallet PVC -- Check operator logs.")
			return reconcile.Result{}, fmt.Errorf("ChiaWalletReconciler ChiaWallet=%s %v", req.NamespacedName, err)
		}

		if pvc != nil {
			res, err = kube.ReconcilePersistentVolumeClaim(ctx, r.Client, wallet.Spec.Storage, *pvc)
			if err != nil {
				r.Recorder.Event(&wallet, corev1.EventTypeWarning, "Failed", "Failed to create wallet PVC -- Check operator logs.")
				return res, fmt.Errorf("ChiaWalletReconciler ChiaWallet=%s %v", req.NamespacedName, err)
			}
		} else {
			return reconcile.Result{}, fmt.Errorf("ChiaWalletReconciler ChiaWallet=%s PVC could not be created", req.NamespacedName)
		}
	}

	// Assemble Deployment
	deploy, err := assembleDeployment(ctx, wallet, networkData)
	if err != nil {
		r.Recorder.Event(&wallet, corev1.EventTypeWarning, "Failed", "Failed to assemble wallet Deployment -- Check operator logs.")
		return reconcile.Result{}, fmt.Errorf("ChiaWalletReconciler ChiaWallet=%s %v", req.NamespacedName, err)
	}
	if err := controllerutil.SetControllerReference(&wallet, &deploy, r.Scheme); err != nil {
		r.Recorder.Event(&wallet, corev1.EventTypeWarning, "Failed", "Failed to assemble wallet Deployment -- Check operator logs.")
		return reconcile.Result{}, fmt.Errorf("ChiaWalletReconciler ChiaWallet=%s %v", req.NamespacedName, err)
	}
	// Reconcile Deployment
	res, err = kube.ReconcileDeployment(ctx, r.Client, deploy)
	if err != nil {
		r.Recorder.Event(&wallet, corev1.EventTypeWarning, "Failed", "Failed to create wallet Deployment -- Check operator logs.")
		return res, fmt.Errorf("ChiaWalletReconciler ChiaWallet=%s %v", req.NamespacedName, err)
	}

	// Update CR status
	r.Recorder.Event(&wallet, corev1.EventTypeNormal, "Created", "Successfully created ChiaWallet resources.")
	wallet.Status.Ready = true
	err = r.Status().Update(ctx, &wallet)
	if err != nil {
		if strings.Contains(err.Error(), kube.ObjectModifiedTryAgainError) {
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		}
		log.Error(err, fmt.Sprintf("ChiaWalletReconciler ChiaWallet=%s unable to update ChiaWallet status", req.NamespacedName))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ChiaWalletReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8schianetv1.ChiaWallet{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.handleChiaNetworks),
		).
		Complete(r)
}

func (r *ChiaWalletReconciler) handleChiaNetworks(ctx context.Context, obj client.Object) []reconcile.Request {
	listOps := &client.ListOptions{
		Namespace: obj.GetNamespace(),
	}
	list := &k8schianetv1.ChiaWalletList{}
	err := r.List(ctx, list, listOps)
	if err != nil {
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, len(list.Items))
	for i, item := range list.Items {
		chiaNetwork := item.Spec.ChiaConfig.ChiaNetwork
		if chiaNetwork != nil && *chiaNetwork == obj.GetName() {
			requests[i] = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      item.GetName(),
					Namespace: item.GetNamespace(),
				},
			}
		}
	}
	return requests
}
