/*
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
*/

package validator

import (
	"crypto/ecdsa"
	"crypto/x509"
	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"
	_ "github.com/openblockchain/obc-peer/openchain"
	"github.com/openblockchain/obc-peer/openchain/crypto/peer"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

// Errors

var (
	ErrRegistrationRequired     error = errors.New("Validator Not Registered to the Membership Service.")
	ErrModuleNotInitialized           = errors.New("Validator Security Module Not Initilized.")
	ErrModuleAlreadyInitialized error = errors.New("Validator Security Module Already Initilized.")

	ErrInvalidTransactionSignature           error = errors.New("Invalid Transaction Signature.")
	ErrTransactionCertificate                error = errors.New("Missing Transaction Certificate.")
	ErrTransactionSignature                  error = errors.New("Missing Transaction Signature.")
	ErrInvalidEncryptedPayloadNilOrEmpty     error = errors.New("Invalid encrypted payload. Nil or empty")
	ErrInvalidEncryptedChaincodeIDNilOrEmpty error = errors.New("Invalid encrypted chaincodeId. Nil or empty")

	ErrEncrypt = errors.New("secret: encryption failed")
	ErrDecrypt = errors.New("secret: decryption failed")
)

var ErrInvalidSignature error = errors.New("Invalid Signature.")

// Log

var log = logging.MustGetLogger("CRYPTO.VALIDATOR")

// Public Struct

type Validator struct {
	*peer.Peer

	isInitialized bool

	rootsCertPool *x509.CertPool

	enrollCerts map[string]*x509.Certificate

	// 48-bytes identifier
	id []byte

	// Enrollment Certificate and private key
	enrollId      string
	enrollCert    *x509.Certificate
	enrollPrivKey interface{}

	// Enrollment Chain
	enrollChainKey []byte
}

type EncryptionScheme interface {
	Encrypt(msg []byte) ([]byte, error)

	Decrypt(ct []byte) ([]byte, error)
}

// Public Methods

// Register is used to register this validator to the membership service.
// The information received from the membership service are stored
// locally and used for initialization.
// This method is supposed to be called only once when the client
// is first deployed.
func (validator *Validator) Register(userId, pwd string) error {
	log.Info("Registering validator [%s]...", userId)

	if err := validator.createKeyStorage(); err != nil {
		log.Error("Failed creating key storage:: %s", err)

		return err
	}

	if err := validator.retrieveECACertsChain(userId); err != nil {
		log.Error("Failed retrieveing ECA certs chain:: %s", err)

		return err
	}

	if err := validator.retrieveTCACertsChain(userId); err != nil {
		log.Error("Failed retrieveing ECA certs chain:: %s", err)

		return err
	}

	if err := validator.retrieveEnrollmentData(userId, pwd); err != nil {
		log.Error("Failed retrieveing enrollment data:: %s", err)

		return err
	}

	log.Info("Registering validator [%s]...done!", userId)

	return nil
}

// Init initializes this validator by loading
// the required certificates and keys which are created at registration time.
// This method must be called at the very beginning to able to use
// the api. If the client is not initialized,
// all the methods will report an error (ErrModuleNotInitialized).
func (validator *Validator) Init() error {
	if validator.isInitialized {
		return ErrModuleAlreadyInitialized
	}

	// Init Conf
	if err := initConf(); err != nil {
		log.Error("Invalid configuration: %s", err)

		return err
	}

	// Initialize DB
	log.Info("Init DB...")
	err := initDB()
	if err != nil {
		if err != ErrDBAlreadyInitialized {
			log.Error("DB already initialized.")
		} else {
			log.Error("Failed initiliazing DB %s", err)

			return err
		}
	}
	log.Info("Init DB...done.")

	// Init crypto engine
	log.Info("Init Crypto Engine...")
	err = validator.initCryptoEngine()
	if err != nil {
		log.Error("Failed initiliazing crypto engine %s", err)
		return err
	}
	log.Info("Init Crypto Engine...done.")

	// Initialisation complete
	validator.isInitialized = true

	return nil
}

// GetID returns this validator's identifier
func (validator *Validator) GetID() []byte {
	// Clone id to avoid exposure of internal data structure
	clone := make([]byte, len(validator.id))
	copy(clone, validator.id)

	return clone
}

// GetEnrollmentID returns this validator's enroolment id
func (validator *Validator) GetEnrollmentID() string {
	return validator.enrollId
}

// TransactionPreValidation verifies that the transaction is
// well formed with the respect to the security layer
// prescriptions (i.e. signature verification).
func (validator *Validator) TransactionPreValidation(tx *obc.Transaction) (*obc.Transaction, error) {
	if !validator.isInitialized {
		return nil, ErrModuleNotInitialized
	}

	if tx.Cert != nil && tx.Signature != nil {
		// TODO: validating signature (and ciphertexts?)

		log.Info("Validating signature...")

		// Verify the transaction
		// 1. Unmarshal cert
		cert, err := utils.DERToX509Certificate(tx.Cert)
		if err != nil {
			log.Error("Failed unmarshalling cert: %s", err)
			return tx, err
		}
		// TODO: verify cert

		// 3. Marshall tx without signature
		signature := tx.Signature
		tx.Signature = nil
		rawTx, err := proto.Marshal(tx)
		if err != nil {
			log.Error("Failed marshaling tx %s:", err)
			return tx, err
		}
		tx.Signature = signature

		// 2. Verify signature
		ok, err := validator.verify(cert.PublicKey, rawTx, tx.Signature)
		if err != nil {
			log.Error("Failed verifying tx signature: %s", err)
			return tx, err
		}
		if !ok {
			return tx, ErrInvalidTransactionSignature
		}

		// Confidentiality
		switch tx.ConfidentialityLevel {
		case obc.Transaction_CHAINCODE_PUBLIC:

			break
		case obc.Transaction_CHAINCODE_CONFIDENTIAL:
			// Check that all the required fields are there.

			if tx.EncryptedPayload == nil || len(tx.EncryptedPayload) == 0 {
				return nil, ErrInvalidEncryptedPayloadNilOrEmpty
			}

			if tx.EncryptedChaincodeID == nil || len(tx.EncryptedChaincodeID) == 0 {
				return nil, ErrInvalidEncryptedChaincodeIDNilOrEmpty
			}

			// TODO: shall we try to decrypt?
		}
	} else {
		if tx.Cert == nil {
			return tx, ErrTransactionCertificate
		}

		if tx.Signature == nil {
			return tx, ErrTransactionSignature
		}
	}

	return tx, nil
}

// TransactionPreValidation verifies that the transaction is
// well formed with the respect to the security layer
// prescriptions (i.e. signature verification). If this is the case,
// the method prepares the transaction to be executed.
func (validator *Validator) TransactionPreExecution(tx *obc.Transaction) (*obc.Transaction, error) {
	if !validator.isInitialized {
		return nil, ErrModuleNotInitialized
	}

	switch tx.ConfidentialityLevel {
	case obc.Transaction_CHAINCODE_PUBLIC:
		// TODO: Nothing to do here?

		break
	case obc.Transaction_CHAINCODE_CONFIDENTIAL:
		// Decrypt payload
		err := validator.decryptTx(tx)
		if err != nil {
			log.Error("Failed decrypting: %s", err)

			return nil, err
		}
	}

	return tx, nil
}

// Sign signs msg with this validator's signing key and outputs
// the signature if no error occurred.
func (validator *Validator) Sign(msg []byte) ([]byte, error) {
	return validator.signWithEnrollmentKey(msg)
}

// Verify checks that signature if a valid signature of message under vkID's verification key.
// If the verification succeeded, Verify returns nil meaning no error occurred.
// If vkID is nil, then the signature is verified against this validator's verification key.
func (validator *Validator) Verify(vkID, signature, message []byte) error {
	cert, err := validator.getEnrollmentCert(vkID)
	if err != nil {
		log.Error("Failed getting enrollment cert for [%s]: %s", utils.EncodeBase64(vkID), err)
	}

	vk := cert.PublicKey.(*ecdsa.PublicKey)

	ok, err := validator.verify(vk, message, signature)
	if err != nil {
		log.Error("Failed verifying signature for [%s]: %s", utils.EncodeBase64(vkID), err)
	}

	if !ok {
		log.Error("Failed invalid signature for [%s]", utils.EncodeBase64(vkID))

		return ErrInvalidSignature
	}

	return nil
}

func (validator *Validator) Close() error {
	getDBHandle().CloseDB()

	return nil
}
